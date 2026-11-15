//go:build windows

// Package drawingpage owns a disposable invisible native diagram page.
package drawingpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/url"
	"path"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/combridge"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

const origin = "https://diagram-picture.invalid"
const wake = 0x8000 + 173

var user32 = windows.NewLazySystemDLL("user32.dll")
var getMessage = user32.NewProc("GetMessageW")
var dispatchMessage = user32.NewProc("DispatchMessageW")
var peekMessage = user32.NewProc("PeekMessageW")
var postThreadMessage = user32.NewProc("PostThreadMessageW")
var ole32 = windows.NewLazySystemDLL("ole32.dll")

type message struct {
	Window         uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

// COM method positions follow the published WebView2.idl interfaces. These
// small bindings supply callbacks and Controller.Close absent from edge's API.
//
//go:uintptrescapes
func call(object uintptr, slot int, args ...uintptr) (uintptr, error) {
	// Native interface addresses remain uintptr. ReadProcessMemory copies the
	// vtable addresses into Go values without a uintptr-to-Pointer conversion.
	// https://learn.microsoft.com/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory
	var table, method uintptr
	if err := windows.ReadProcessMemory(windows.CurrentProcess(), object, (*byte)(unsafe.Pointer(&table)), unsafe.Sizeof(table), nil); err != nil {
		return 0, err
	}
	if err := windows.ReadProcessMemory(windows.CurrentProcess(), table+uintptr(slot)*unsafe.Sizeof(method), (*byte)(unsafe.Pointer(&method)), unsafe.Sizeof(method), nil); err != nil {
		return 0, err
	}
	result, _, _ := syscall.SyscallN(method, append([]uintptr{object}, args...)...)
	if int32(result) < 0 {
		return result, fmt.Errorf("WebView2 HRESULT 0x%08x", uint32(result))
	}
	return result, nil
}

type completed interface {
	Invoke(uintptr, unsafe.Pointer) uintptr
}
type callback struct {
	f func(uintptr, unsafe.Pointer) uintptr
}

func (c *callback) Invoke(code uintptr, object unsafe.Pointer) uintptr { return c.f(code, object) }

type eventHandler interface {
	InvokeEvent(unsafe.Pointer, unsafe.Pointer) uintptr
}
type eventCallback struct {
	f func(unsafe.Pointer, unsafe.Pointer) uintptr
}

func (c *eventCallback) InvokeEvent(sender, args unsafe.Pointer) uintptr { return c.f(sender, args) }
func init() {
	combridge.RegisterVTable[combridge.IUnknown, completed]("{6C4819F3-C9B7-4260-8127-C9F5BDE7F68C}", func(this, code uintptr, object unsafe.Pointer) uintptr {
		return combridge.Resolve[completed](this).Invoke(code, object)
	})
	combridge.RegisterVTable[combridge.IUnknown, eventHandler]("{57213F19-00E6-49FA-8E07-898EA01ECBD2}", func(this uintptr, sender, args unsafe.Pointer) uintptr {
		return combridge.Resolve[eventHandler](this).InvokeEvent(sender, args)
	})
}

// Each event interface has its own IID, even when their Invoke ABI is alike.
type resourceHandler interface {
	Resource(unsafe.Pointer, unsafe.Pointer) uintptr
}
type resourceCallback struct {
	f func(unsafe.Pointer, unsafe.Pointer) uintptr
}

func (c *resourceCallback) Resource(sender, args unsafe.Pointer) uintptr { return c.f(sender, args) }
func init() {
	combridge.RegisterVTable[combridge.IUnknown, resourceHandler]("{AB00B74C-15F1-4646-80E8-E76341D25D71}", func(this uintptr, sender, args unsafe.Pointer) uintptr {
		return combridge.Resolve[resourceHandler](this).Resource(sender, args)
	})
}

type environmentDone struct {
	f func(webviewloader.HRESULT, *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT
}

func (e *environmentDone) EnvironmentCompleted(code webviewloader.HRESULT, env *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
	return e.f(code, env)
}

// Draw does not expose a window, network service, lease or general script API.
func Draw(ctx context.Context, assets fs.FS, request Request, options Options) (Picture, error) {
	if request.Kind != "canvas" && request.Kind != "mermaid" {
		return Picture{}, errors.New("unsupported diagram kind")
	}
	if request.Scale < 1 || request.Scale > 3 || len(request.Text) > 2<<20 {
		return Picture{}, errors.New("diagram request exceeds its limits")
	}
	return draw(ctx, assets, options, scriptFor(request))
}

func scriptFor(request Request) string {
	data, _ := json.Marshal(request)
	script := `Promise.resolve().then(async function(){
 if(!document.createElement('canvas').getContext('2d')) throw Error('A 2D canvas is required.');
 const request=` + string(data) + `;
	 await document.fonts.load('14px Inter',request.text);
	 await document.fonts.load('600 14px Inter',request.text);
	 const supplied=request.images||{};
	 request.pictures=async function(name){
	  if(typeof name!=='string'||name.startsWith('/')||/[\\:\x00-\x1f]/.test(name)) throw Error('The image path is invalid.');
	  const parts=(request.sourcePath||'').split('/');parts.pop();
	  for(const part of name.split('/')){if(part==='..'){if(!parts.length)throw Error('The image leaves the workspace.');parts.pop();}else if(part&&part!=='.'){parts.push(part);}}
	  const key=parts.join('/');if(!Object.prototype.hasOwnProperty.call(supplied,key))throw Error('An image was not declared in the snapshot.');
	  return {bytes:Uint8Array.from(atob(supplied[key]),c=>c.charCodeAt(0))};
	 };
 const measured=await yavcDiagramPicture.draw(Object.assign({},request,{format:'svg'}));
 if(measured.problems.length) throw Error(measured.problems.join('; '));
 const width=Math.round(measured.width*request.scale),height=Math.round(measured.height*request.scale);
 if(width>8192||height>8192||width*height>16777216) throw Error('The diagram exceeds the picture dimensions.');
 const out=await yavcDiagramPicture.draw(Object.assign({},request,{format:'png'}));
 if(out.problems.length) throw Error(out.problems.join('; '));
 const data=await new Promise((resolve,reject)=>{const r=new FileReader();r.onload=()=>resolve(r.result.split(',')[1]);r.onerror=reject;r.readAsDataURL(out.blob);});
 chrome.webview.postMessage(JSON.stringify({data,width:out.width,height:out.height}));
 }).catch(e=>chrome.webview.postMessage(JSON.stringify({error:String(e.message||e)})));`
	return script
}

func draw(ctx context.Context, assets fs.FS, options Options, script string) (Picture, error) {
	if err := ctx.Err(); err != nil {
		return Picture{}, err
	}
	if options.DataFolder == "" {
		return Picture{}, errors.New("a private browser data folder is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	type answer struct {
		picture Picture
		err     error
	}
	result := make(chan answer, 1)
	ready := make(chan struct{})
	var once sync.Once
	markReady := func() { once.Do(func() { close(ready) }) }
	go func() { p, err := native(ctx, assets, options, script, markReady); result <- answer{p, err} }()
	select {
	case done := <-result:
		return done.picture, done.err
	case <-ctx.Done():
		select {
		case <-ready:
			done := <-result
			return done.picture, done.err
		default:
			settled := make(chan error, 1)
			go func() {
				done := <-result
				var disposal *DisposalError
				if errors.As(done.err, &disposal) {
					settled <- disposal
				} else {
					settled <- nil
				}
				close(settled)
			}()
			return Picture{}, &StartupError{Settled: settled}
		}
	}
}

func native(ctx context.Context, assets fs.FS, options Options, script string, markReady func()) (Picture, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return Picture{}, fmt.Errorf("%w: initialize COM 0x%x", ErrUnavailable, hr)
	}
	defer ole32.NewProc("CoUninitialize").Call()
	emit := func(s string) {
		if options.Event != nil {
			options.Event(s)
		}
	}
	var msg message
	peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
	thread := windows.GetCurrentThreadId()
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			postThreadMessage.Call(uintptr(thread), wake, 0, 0)
		case <-stopped:
		}
	}()
	var controller *edge.ICoreWebView2Controller
	var webview *edge.ICoreWebView2
	var environment *edge.ICoreWebView2Environment
	var terminal bool
	var result Picture
	var failure error
	creationFinished := false
	finishCreation := func() { creationFinished = true; markReady() }
	finish := func(err error) {
		if !terminal {
			terminal = true
			failure = err
			postThreadMessage.Call(uintptr(thread), wake, 0, 0)
		}
	}
	var pendingController func()
	var startupGate <-chan struct{}
	controllerCallback := combridge.New[completed](&callback{f: func(code uintptr, object unsafe.Pointer) uintptr {
		if object != nil {
			controller = (*edge.ICoreWebView2Controller)(object)
			controller.AddRef()
		}
		complete := func() {
			finishCreation()
			if int32(code) < 0 || object == nil {
				finish(fmt.Errorf("%w: create invisible controller 0x%x", ErrUnavailable, code))
				return
			}
			emit("controller-created")
			if ctx.Err() != nil {
				finish(ctx.Err())
				return
			}
			if err := controller.PutIsVisible(false); err != nil {
				finish(err)
				return
			}
			if err := controller.PutBounds(edge.Rect{Right: 1200, Bottom: 900}); err != nil {
				finish(err)
				return
			}
			emit("controller-hidden")
			var err error
			webview, err = controller.GetCoreWebView2()
			if err != nil {
				finish(err)
				return
			}
			settings, err := webview.GetSettings()
			if err != nil {
				finish(err)
				return
			}
			settings.PutIsScriptEnabled(true)
			settings.PutIsWebMessageEnabled(true)
			settings.PutAreDevToolsEnabled(false)
			settings.PutAreDefaultScriptDialogsEnabled(false)
			settings.Release()
		}
		if delayStartupCompletion != nil {
			startupGate = delayStartupCompletion()
		}
		if startupGate != nil {
			pendingController = complete
			emit("startup-held")
			go func() {
				select {
				case <-startupGate:
					postThreadMessage.Call(uintptr(thread), wake, 0, 0)
				case <-stopped:
				}
			}()
		} else {
			complete()
		}
		return 0
	}})
	defer controllerCallback.Close()
	webMessage := combridge.New[eventHandler](&eventCallback{f: func(sender, args unsafe.Pointer) uintptr {
		if terminal {
			return 0
		}
		raw, err := (*edge.ICoreWebView2WebMessageReceivedEventArgs)(args).TryGetWebMessageAsString()
		if err != nil {
			finish(err)
			return 0
		}
		var message struct {
			Data          string `json:"data"`
			Width, Height int
			Error         string `json:"error"`
			Started       bool   `json:"started"`
		}
		if err = json.Unmarshal([]byte(raw), &message); err != nil {
			finish(err)
			return 0
		}
		if message.Started {
			emit("script-started")
			return 0
		}
		if message.Error != "" {
			finish(errors.New(message.Error))
			return 0
		}
		if len(message.Data) > 24<<20 {
			finish(errors.New("picture response too large"))
			return 0
		}
		result.PNG, err = base64.StdEncoding.DecodeString(message.Data)
		result.Width = message.Width
		result.Height = message.Height
		if err == nil {
			err = validatePNG(result)
		}
		finish(err)
		return 0
	}})
	defer webMessage.Close()
	html := `<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'self'; img-src data: blob:; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'"><link rel="stylesheet" href="/pudl/pudl.css"><script src="/errors.js"></script><script src="/yavc/support/yavc-util.js"></script><script src="/yavc/content/diagram-kit/picture.js"></script><body><script src="/draw.js"></script>`
	resources := combridge.New[resourceHandler](&resourceCallback{f: func(sender, args unsafe.Pointer) uintptr {
		event := (*edge.ICoreWebView2WebResourceRequestedEventArgs)(args)
		request, err := event.GetRequest()
		if err != nil {
			finish(err)
			return 0
		}
		defer request.Release()
		uri, err := request.GetUri()
		if err != nil {
			finish(err)
			return 0
		}
		u, err := url.Parse(uri)
		if err == nil {
			emit("asset:" + u.Path)
		}
		var body []byte
		status := 200
		contentType := "text/javascript"
		if err != nil || u.Scheme+"://"+u.Host != origin {
			status = 403
		} else {
			switch u.Path {
			case "/errors.js":
				body = []byte(`window.addEventListener('error',e=>chrome.webview.postMessage(JSON.stringify({error:e.message||'Drawing asset failed'})));window.addEventListener('unhandledrejection',e=>chrome.webview.postMessage(JSON.stringify({error:String(e.reason)})));`)
			case "/":
				body = []byte(html)
				contentType = "text/html"
			case "/draw.js":
				body = []byte(script)
			default:
				file := strings.TrimPrefix(u.Path, "/")
				allowed := file == "pudl/pudl.css" || strings.HasPrefix(file, "pudl/fonts/") || strings.HasPrefix(file, "yavc/content/diagram-kit/") || strings.HasPrefix(file, "yavc/content/md-editor/") || file == "yavc/content/md-engine/safe-html.js" || file == "yavc/support/yavc-util.js" || strings.HasPrefix(file, "yavc/support/vendor/")
				if !allowed || !fs.ValidPath(file) {
					status = 403
				} else {
					body, err = fs.ReadFile(assets, file)
					if err != nil {
						status = 404
					}
					if found := mime.TypeByExtension(path.Ext(file)); found != "" {
						contentType = found
					}
				}
			}
		}
		if status != 200 {
			body = nil
			emit(fmt.Sprintf("resource-refused:%s:%d", uri, status))
			if u == nil || u.Path != "/favicon.ico" {
				finish(errors.New("a drawing resource was refused"))
			}
		}
		response, err := responseFor(environment, body, status, "Content-Type: "+contentType+"\r\nCache-Control: no-store")
		if err != nil {
			finish(err)
			return 0
		}
		defer response.Release()
		if err = event.PutResponse(response); err != nil {
			finish(err)
		}
		return 0
	}})
	defer resources.Close()
	setup := false
	var releaseRestrictions func()
	defer func() {
		if releaseRestrictions != nil {
			releaseRestrictions()
		}
	}()
	envHandler := &environmentDone{f: func(code webviewloader.HRESULT, env *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
		if code < 0 || env == nil {
			finishCreation()
			finish(fmt.Errorf("%w: create invisible environment 0x%x", ErrUnavailable, uint32(code)))
			return 0
		}
		env.AddRef()
		environment = (*edge.ICoreWebView2Environment)(unsafe.Pointer(env))
		if ctx.Err() != nil {
			finishCreation()
			finish(ctx.Err())
			return 0
		}
		// HWND_MESSAGE is explicitly supported for invisible WebViews.
		if _, err := call(uintptr(unsafe.Pointer(env)), 3, ^uintptr(2), controllerCallback.Ref()); err != nil {
			finishCreation()
			finish(err)
		}
		return 0
	}}
	err := webviewloader.CreateCoreWebView2EnvironmentWithOptions(envHandler, webviewloader.WithUserDataFolder(options.DataFolder))
	if err != nil {
		return Picture{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	for !terminal {
		if pendingController != nil {
			select {
			case <-startupGate:
				f := pendingController
				pendingController = nil
				f()
			default:
			}
		}
		if ctx.Err() != nil && creationFinished {
			finish(ctx.Err())
			break
		}
		if webview != nil && !setup {
			setup = true
			releaseRestrictions, err = restrictPage(webview, emit, finish)
			if err != nil {
				finish(err)
				break
			}
			var token int64
			if _, err = call(uintptr(unsafe.Pointer(webview)), 34, webMessage.Ref(), uintptr(unsafe.Pointer(&token))); err != nil {
				finish(err)
				break
			}
			if _, err = call(uintptr(unsafe.Pointer(webview)), 55, resources.Ref(), uintptr(unsafe.Pointer(&token))); err != nil {
				finish(err)
				break
			}
			if err = webview.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL); err != nil {
				finish(err)
				break
			}
			if err = webview.Navigate(origin + "/"); err != nil {
				finish(err)
				break
			}
		}
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			finish(errors.New("native page message loop ended"))
			break
		}
		if msg.ID != wake {
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
	if controller != nil {
		closeErr := closeNativeController(unsafe.Pointer(controller))
		if closeErr != nil {
			failure = &DisposalError{Err: closeErr}
		} else {
			emit("controller-closed")
			// This observation confirms native closure. Correctness rests on
			// Controller.Close, not on an assumption about GetCoreWebView2.
			if webview != nil && webview.ExecuteScript("void 0", nil) != nil {
				emit("closed-controller-refused")
			}
		}
		controller.Release()
	}
	if webview != nil {
		webview.Release()
	}
	if environment != nil {
		call(uintptr(unsafe.Pointer(environment)), 2)
	}
	runtime.KeepAlive(envHandler)
	if failure != nil {
		return Picture{}, failure
	}
	return result, nil
}
