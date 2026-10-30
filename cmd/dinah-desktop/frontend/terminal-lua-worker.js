/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   A Lua script of the Terminal runs here, in a worker of its own, so that a script that loops forever
   cannot hold up the page, and Ctrl+C ends it by ending the worker. Lua is
   wasmoon (vendor/wasmoon-1.16.0): the reference Lua 5.4 compiled to
   WebAssembly. The script reaches nothing but what the Terminal answers:
   every file, command, variable and applet is a message to the page, and
   the page's answer is a promise the script waits on.

   The page sends { type: 'run', source, name, args, stdin, wasm, dir }; the
   worker sends { type: 'out' | 'err', text } as the script writes,
   { type: 'call', id, op, args } for anything it needs, and
   { type: 'done', status } at the end; the page answers each call with
   { type: 'reply', id, ok, value }. */
'use strict';
importScripts('vendor/wasmoon-1.16.0/index.js');

var pending = new Map(), next = 0;
function post(m) { self.postMessage(m); }
function call(op, args) {
  return new Promise(function (resolve, reject) {
    var id = ++next;
    pending.set(id, { resolve: resolve, reject: reject });
    post({ type: 'call', id: id, op: op, args: args });
  });
}

/* The Lua side: the standard functions a script uses, wired to the page
   through __host (a call that returns a promise), __out and __err. A
   function that waits is a Lua function, so that waiting never crosses a
   C function, which Lua does not allow. */
var PRELUDE = [
  "local host_call, out, err, done = __host, __out, __err, __done",
  "__host, __out, __err, __done = nil, nil, nil, nil",
  "local function host(op, ...) return host_call(op, ...):await() end",
  "local EXIT = {}",
  "local function text_of(...)",
  "  local parts = {}",
  "  for i = 1, select('#', ...) do local v = select(i, ...) parts[i] = type(v) == 'number' and tostring(v) or v end",
  "  return table.concat(parts)",
  "end",
  "function print(...)",
  "  local parts = {}",
  "  for i = 1, select('#', ...) do parts[i] = tostring((select(i, ...))) end",
  "  out(table.concat(parts, '\\t') .. '\\n')",
  "end",
  "",
  "-- A reader over text, for stdin, a file read whole and a command's output.",
  "local function reader(text)",
  "  local pos = 1",
  "  local r = {}",
  "  local function one(fmt)",
  "    if type(fmt) == 'number' then",
  "      if pos > #text then return nil end",
  "      local s = text:sub(pos, pos + fmt - 1) pos = pos + #s return s",
  "    end",
  "    fmt = tostring(fmt or 'l'):gsub('^%*', '')",
  "    local f = fmt:sub(1, 1)",
  "    if f == 'a' then local s = text:sub(pos) pos = #text + 1 return s end",
  "    if f == 'n' then",
  "      local s, e = text:find('^%s*[-+]?%d*%.?%d+[eE]?[-+]?%d*', pos)",
  "      if not s then return nil end",
  "      local v = tonumber(text:sub(s, e):match('%S.*')) pos = e + 1 return v",
  "    end",
  "    if pos > #text then return nil end",
  "    local e = text:find('\\n', pos, true)",
  "    local line",
  "    if e then line = text:sub(pos, f == 'L' and e or e - 1) pos = e + 1",
  "    else line = text:sub(pos) pos = #text + 1 end",
  "    return line",
  "  end",
  "  function r.read(...)",
  "    local n = select('#', ...)",
  "    if n == 0 then return one('l') end",
  "    local res = {}",
  "    for i = 1, n do res[i] = one((select(i, ...))) if res[i] == nil then return table.unpack(res, 1, i) end end",
  "    return table.unpack(res, 1, n)",
  "  end",
  "  function r.lines(...) local fmts = table.pack(...) return function() return r.read(table.unpack(fmts, 1, fmts.n)) end end",
  "  return r",
  "end",
  "",
  "local File = {}",
  "File.__index = File",
  "File.__name = 'FILE*'",
  "function File:read(...) if not self.r then return nil, 'the file is not open for reading' end return self.r.read(...) end",
  "function File:lines(...) if not self.r then error('the file is not open for reading', 2) end return self.r.lines(...) end",
  "function File:write(...)",
  "  if not self.mode or self.mode == 'r' then return nil, 'the file is not open for writing' end",
  "  if self.sink then self.sink(text_of(...)) return self end",
  "  self.buffer[#self.buffer + 1] = text_of(...)",
  "  return self",
  "end",
  "function File:flush()",
  "  if self.sink then self.sink(table.concat(self.buffer)) self.buffer = {} return self end",
  "  if self.path and self.mode ~= 'r' and #self.buffer > 0 then",
  "    host('save', self.path, table.concat(self.buffer), self.append and 'append' or 'replace')",
  "    self.buffer, self.append = {}, true",
  "  end",
  "  return self",
  "end",
  "function File:close()",
  "  if self.closed then return nil, 'the file is already closed' end",
  "  self:flush() self.closed = true",
  "  if self.status then if self.status == 0 then return true, 'exit', 0 end return nil, 'exit', self.status end",
  "  return true",
  "end",
  "function File:seek() return nil, 'seek is not available here' end",
  "function File:setvbuf() return true end",
  "File.__close = File.close",
  "File.__tostring = function(f) return 'file (' .. (f.path or 'standard') .. ')' end",
  "",
  "local stdin = __stdin",
  "__stdin = nil",
  "io.stdin = setmetatable({ r = reader(stdin or ''), mode = 'r', path = 'stdin' }, File)",
  "io.stdout = setmetatable({ mode = 'w', buffer = {}, sink = out, path = 'stdout' }, File)",
  "io.stderr = setmetatable({ mode = 'w', buffer = {}, sink = err, path = 'stderr' }, File)",
  "local current_in, current_out = io.stdin, io.stdout",
  "function io.write(...) out(text_of(...)) return io.stdout end",
  "function io.read(...) return current_in:read(...) end",
  "function io.open(path, mode)",
  "  mode = (mode or 'r'):gsub('b', '')",
  "  local m = mode:sub(1, 1)",
  "  if m ~= 'r' and m ~= 'w' and m ~= 'a' then error(\"invalid mode '\" .. mode .. \"'\", 2) end",
  "  local f = setmetatable({ path = path, mode = m, buffer = {}, append = m == 'a' }, File)",
  "  if m == 'r' or mode:find('+', 1, true) then",
  "    local ok, text = pcall(host, 'read', path)",
  "    if not ok then",
  "      if m == 'r' then return nil, path .. ': ' .. tostring(text):gsub('^Error: ', ''), 2 end",
  "      text = ''",
  "    end",
  "    f.r = reader(text)",
  "  end",
  "  if m == 'w' then host('save', path, '', 'replace') f.append = true end",
  "  return f",
  "end",
  "function io.lines(path, ...)",
  "  if path == nil then return current_in:lines(...) end",
  "  local f, msg = io.open(path, 'r')",
  "  if not f then error(msg, 2) end",
  "  return f:lines(...)",
  "end",
  "function io.input(f) if f then current_in = type(f) == 'string' and assert(io.open(f, 'r')) or f end return current_in end",
  "function io.output(f) if f then current_out = type(f) == 'string' and assert(io.open(f, 'w')) or f end return current_out end",
  "function io.close(f) return (f or current_out):close() end",
  "function io.type(f) if getmetatable(f) == File then return f.closed and 'closed file' or 'file' end return nil end",
  "function io.popen(command, mode)",
  "  if mode and mode:sub(1, 1) ~= 'r' then error('io.popen can only read a command\\'s output here', 2) end",
  "  local result = host('exec', command)",
  "  local nl = result:find('\\n', 1, true)",
  "  local f = setmetatable({ path = command, mode = 'r', buffer = {}, status = tonumber(result:sub(1, nl - 1)) }, File)",
  "  f.r = reader(result:sub(nl + 1))",
  "  return f",
  "end",
  "io.tmpfile = function() return nil, 'temporary files are not available here' end",
  "",
  "function os.execute(command)",
  "  if command == nil then return true end",
  "  local status = host('run', command)",
  "  if status == 0 then return true, 'exit', 0 end",
  "  return nil, 'exit', status",
  "end",
  "function os.getenv(name) local v = host('getenv', name) if v == '' then return nil end return v end",
  "function os.exit(code) if code == true or code == nil then code = 0 elseif code == false then code = 1 end error(setmetatable({ code = math.tointeger(code) or 1 }, EXIT), 0) end",
  "function os.remove(path) local ok, e = pcall(host, 'remove', path) if ok then return true end return nil, path .. ': ' .. tostring(e):gsub('^Error: ', ''), 2 end",
  "function os.rename(a, b) local ok, e = pcall(host, 'move', a, b) if ok then return true end return nil, tostring(e):gsub('^Error: ', ''), 2 end",
  "os.tmpname = function() error('temporary files are not available here', 2) end",
  "",
  "function loadfile(path, mode, env)",
  "  local ok, text = pcall(host, 'read', path)",
  "  if not ok then return nil, path .. ': ' .. tostring(text):gsub('^Error: ', '') end",
  "  return load(text, '@' .. path, mode, env)",
  "end",
  "function dofile(path) local f = assert(loadfile(path)) return f() end",
  "",
  "-- require finds a module in the script's folder, ~/lib/lua and /lib/lua.",
  "local loaded = package.loaded",
  "function require(name)",
  "  if loaded[name] ~= nil then return loaded[name] end",
  "  if package.preload[name] then local v = package.preload[name](name) if v == nil then v = true end loaded[name] = v return v end",
  "  local found = host('module', name)",
  "  if found == '' then error(\"module '\" .. name .. \"' not found in the script's folder, ~/lib/lua or /lib/lua\", 2) end",
  "  local nl = found:find('\\n', 1, true)",
  "  local path = found:sub(1, nl - 1)",
  "  local chunk, e = load(found:sub(nl + 1), '@' .. path)",
  "  if not chunk then error(e, 2) end",
  "  local v = chunk(name, path)",
  "  if v == nil then v = loaded[name] == nil and true or loaded[name] end",
  "  loaded[name] = v",
  "  return v",
  "end",
  "",
  "-- fs: Dinah's file system beyond what io and os give.",
  "package.preload.fs = function()",
  "  local json = require 'json'",
  "  return {",
  "    list = function(path) return json.decode(host('list', path or '.')) end,",
  "    stat = function(path) local s = host('stat', path) if s == '' then return nil end return json.decode(s) end,",
  "    exists = function(path) return host('stat', path) ~= '' end,",
  "    mkdir = function(path, parents) host('mkdir', path, parents and true or false) return true end,",
  "    cwd = function() return host('cwd') end,",
  "    path = function(path) return host('path', path) end,",
  "  }",
  "end",
  "",
  "-- applets: the actions of Dinah's applets, as the Terminal's",
  "-- applets command offers them. Input and output are Lua tables; invoke",
  "-- waits for the action to end, and a failed action raises an error.",
  "package.preload.applets = function()",
  "  local json = require 'json'",
  "  local catalog",
  "  local function load_catalog() if not catalog then catalog = json.decode(host('applets')) end return catalog end",
  "  return {",
  "    list = function()",
  "      local out = {}",
  "      for i, a in ipairs(load_catalog()) do out[i] = { id = a.id, title = a.title } end",
  "      return out",
  "    end,",
  "    actions = function(id)",
  "      local out = {}",
  "      for _, a in ipairs(load_catalog()) do",
  "        if not id or a.id == id then for _, act in ipairs(a.actions or {}) do out[#out + 1] = act end end",
  "      end",
  "      return out",
  "    end,",
  "    invoke = function(action, input)",
  "      -- An action's input is always an object, even when it is empty.",
  "      local text = host('invoke', action, json.encode(json.object(input or {})))",
  "      local ok, value = pcall(json.decode, text)",
  "      if ok then return value end",
  "      return text",
  "    end,",
  "  }",
  "end",
  "",
  "-- editor: Dinah's Editor. open_draft opens a draft in progress,",
  "-- by the id dinah.documents/list@1 gives it, in an Editor window.",
  "package.preload.editor = function()",
  "  return { open_draft = function(id) host('open-draft', id) return true end }",
  "end",
  "",
  "-- log: entries in Dinah's log, under script:<the script's name>.",
  "package.preload.log = function()",
  "  local source = 'script:' .. (tostring(arg and arg[0] or 'lua'):match('([^/]+)$') or 'lua'):gsub('[^%w._@-]', '-'):sub(1, 128)",
  "  local function entry(level) return function(...) local parts = {} for i = 1, select('#', ...) do parts[i] = tostring((select(i, ...))) end host('log', source, level, table.concat(parts, ' ')) end end",
  "  return { info = entry('info'), warn = entry('warning'), error = entry('error') }",
  "end",
  "",
  "-- Each argument comes as a global of its own, since text passed into Lua",
  "-- ends at its first NUL.",
  "local args = {}",
  "for i = 1, __argn do args[i] = _G['__arg' .. i] _G['__arg' .. i] = nil end",
  "arg = { [0] = __name }",
  "for i, a in ipairs(args) do arg[i] = a end",
  "local source, name = __source, __name",
  "__source, __name, __argn = nil, nil, nil",
  "if source:sub(1, 2) == '#!' then source = '--' .. source end",
  "local chunk, e = load(source, '@' .. name)",
  "if not chunk then err(tostring(e) .. '\\n') done(1) return end",
  "local ok, problem = xpcall(chunk, function(m)",
  "  if getmetatable(m) == EXIT then return m end",
  "  return debug.traceback(tostring(m), 2)",
  "end, table.unpack(args))",
  "io.stdout:flush()",
  "if ok then done(0) return end",
  "if getmetatable(problem) == EXIT then done(problem.code) return end",
  "err(tostring(problem) .. '\\n')",
  "done(1)"
].join('\n');

self.onmessage = function (e) {
  var m = e.data;
  if (m.type === 'reply') {
    var p = pending.get(m.id);
    if (!p) return;
    pending.delete(m.id);
    if (m.ok) p.resolve(m.value); else p.reject(new Error(m.value));
    return;
  }
  if (m.type !== 'run') return;
  (async function () {
    var factory = new wasmoon.LuaFactory(m.wasm);
    var lua = await factory.createEngine({ injectObjects: false });
    lua.global.set('__host', function (op) { return call(op, Array.prototype.slice.call(arguments, 1)); });
    lua.global.set('__out', function (s) { post({ type: 'out', text: String(s) }); });
    lua.global.set('__err', function (s) { post({ type: 'err', text: String(s) }); });
    lua.global.set('__done', function (status) { post({ type: 'done', status: Number(status) || 0 }); });
    lua.global.set('__source', m.source);
    lua.global.set('__name', m.name);
    var args = m.args || [];
    lua.global.set('__argn', args.length);
    args.forEach(function (a, i) { lua.global.set('__arg' + (i + 1), String(a)); });
    lua.global.set('__stdin', m.stdin == null ? undefined : m.stdin);
    await lua.doString(PRELUDE);
  })().catch(function (err) {
    post({ type: 'err', text: String(err && err.message || err) + '\n' });
    post({ type: 'done', status: 1 });
  });
};
