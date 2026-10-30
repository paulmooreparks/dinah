/* Devin's own part of Settings, Agents: what it may do without asking, and the address of its usage
   page. It is the provider's own surface, so it names the provider and keeps its own markup and
   calls; the Settings page mounts it when the engine's list has Devin in it (settings-providers.js). */
(function () {
  'use strict';

  function mount(root, api) {
    var product = window.dinahProduct ? window.dinahProduct.name : '';
    root.innerHTML = `<section class="card vstack gap-3" aria-labelledby="set-approvals">
  <h3 class="card-title" id="set-approvals">Devin Approvals</h3>
  <p class="form-help">These settings apply only to Devin. A change applies when Devin next starts.</p>
  <div class="vstack gap-2" role="radiogroup" aria-labelledby="set-approvals">
    <label class="check"><input type="radio" name="approvals" value="devin"> <span><strong>Use My Own Devin Settings</strong> unchanged, so Devin asks what it asks in Devin Desktop.</span></label>
    <label class="check"><input type="radio" name="approvals" value="all"> <span><strong>Approve Everything</strong> and do not ask. Devin is still never allowed to read or set your tokens.</span></label>
    <label class="check"><input type="radio" name="approvals" value="dinah"> <span><strong>Allow Reading and Looking</strong> and ask before anything else. Reading files, searching, read-only shell commands and making folders go ahead.</span></label>
    <label class="check"><input type="radio" name="approvals" value="ask"> <span><strong>Ask Before Every Action</strong> that is not a read.</span></label>
  </div>
</section>
<section class="card vstack gap-3" aria-labelledby="set-usage">
  <h3 class="card-title" id="set-usage">Devin Usage Page</h3>
  <p class="form-help" id="usage-help">The usage page opened for Devin, such as the My analytics page of your Devin web app. Other providers use their own usage pages.</p>
  <label class="form-label" for="usage-page">Usage Page Address</label>
  <input class="form-input" id="usage-page" type="url" autocomplete="off" spellcheck="false">
  <div class="settings-actions"><button type="button" class="btn btn-sm" id="usage-save">Save</button></div>
</section>`;
    function $(id) { return root.querySelector('#' + id); }
    var app = api.app, toast = api.toast, fail = api.fail;

    root.querySelectorAll('input[name="approvals"]').forEach(function (r) {
      r.addEventListener('change', function () {
        if (!r.checked) return;
        app().SetApprovals(r.value).then(function () { toast('The new approvals apply when Devin next starts. Restart Devin, in the Devin menu at the top right, applies them now.'); });
      });
    });
    function ticks(level) {
      root.querySelectorAll('input[name="approvals"]').forEach(function (r) { r.checked = r.value === level; });
    }

    // The usage page's address; an empty field clears it.
    function usagePage(state) { $('usage-page').value = state.usagePage || ''; }
    $('usage-save').addEventListener('click', function () {
      app().SetUsagePage($('usage-page').value).then(function () { return app().State(); }).then(function (state) {
        usagePage(state);
        toast(state.usagePage ? 'The Devin usage page is now ' + state.usagePage + '.' : 'The Devin usage page is cleared.', 'positive');
      }).catch(fail);
    });

    // What State says, at start and after each save.
    return {
      apply: function (state) {
        ticks(state.approvals);
        usagePage(state);
      }
    };
  }

  window.dinahProviderSettings.register('devin', {
    mount: mount,
    // Devin's model prices are read from its command line, which says so under its row.
    row: function (container) {
      var prices = document.createElement('p'); prices.className = 'form-help devin-prices-note';
      prices.textContent = 'Model prices in the model list come from the Devin command line, and may be missing if Devin changes what it prints.';
      container.appendChild(prices);
    }
  });
})();
