(function () {
  'use strict';
  var app=window.dinahHelpers.app(),current=null,finished=new Set();
  function close(id){finished.add(id);if(!current||current.id!==id)return;var dialog=current.dialog;current=null;dialog.close();dialog.remove();}
  function fail(error){document.dispatchEvent(new CustomEvent('dinah:menu-error',{detail:error}));}
  function confirmation(request){
    if(!request?.id||!request.status||finished.has(request.id))return;
    if(current?.id===request.id){current.dialog.querySelector('[data-cancel]').focus();return;}
    if(current)close(current.id);
    var dialog=document.createElement('dialog');dialog.className='dialog exit-confirmation';
    dialog.setAttribute('aria-labelledby','exit-confirmation-title');dialog.setAttribute('aria-describedby','exit-confirmation-detail');
    dialog.innerHTML='<h2 class="dialog-title" id="exit-confirmation-title"></h2><div class="dialog-body" id="exit-confirmation-detail"><p>Work is still in progress.</p><ul></ul><p>Exiting will interrupt this work. It will not restart automatically. Saved chats can be reopened.</p></div><div class="dialog-actions"><button type="button" class="btn" data-cancel autofocus>Cancel</button><button type="button" class="btn btn-danger" data-exit></button></div>';
    // The product's name (product.js) goes in as text.
    var name=window.dinahProduct?window.dinahProduct.name:'';
    dialog.querySelector('#exit-confirmation-title').textContent='Exit '+name+'?';dialog.querySelector('[data-exit]').textContent='Exit '+name;
    var status=request.status,list=dialog.querySelector('ul');
    function item(text){var li=document.createElement('li');li.textContent=text;list.append(li);}
    if(status.chats>0)item('Active chats: '+status.chats+'. Their current turns will stop.');
    if(status.terminals>0)item('Native terminal sessions: '+status.terminals+'. Their processes will end.');
    if(status.nonResumableActions||status.unfinishedRequests>0)item('Actions or commands have not finished. Some changes may already have been saved.');
    var answering=false;
    async function answer(accepted){
      if(answering)return;answering=true;
      dialog.querySelectorAll('button').forEach(function(button){button.disabled=true;});
      try{await app.AnswerExitConfirmation(request.id,accepted);close(request.id);}
      catch(error){close(request.id);fail(error);}
    }
    dialog.querySelector('[data-cancel]').onclick=function(){answer(false);};
    dialog.querySelector('[data-exit]').onclick=function(){answer(true);};
    dialog.addEventListener('cancel',function(event){event.preventDefault();answer(false);});
    current={id:request.id,dialog:dialog};document.body.append(dialog);dialog.showModal();dialog.querySelector('[data-cancel]').focus();
  }
  window.dinahHost.on('dinah:exit-confirmation',confirmation);
  window.dinahHost.on('dinah:exit-confirmation-resolved',function(request){close(request.id);});
  // After a resync the engine's open confirmation is read again: shown if the page has not
  // seen it, and a shown one that is no longer open was resolved or expired unseen.
  window.dinahHost.onResync(function(){
    if(!app.PendingExitConfirmation)return null;
    return Promise.resolve(app.PendingExitConfirmation()).then(function(request){
      if(request&&request.id)confirmation(request);
      else if(current)close(current.id);
    });
  });
  window.dinahHost.on('dinah:exit-confirmation-expired',function(request){close(request.id);fail(Error('Exit was cancelled because the confirmation was not answered.'));});
})();
