function setupMockup() {
  const preview = document.querySelector('.preview'), layer = document.querySelector('[data-win-layer]');
  function status() {
    const width = Math.round(layer.getBoundingClientRect().width), restricted = pudlWindows.effectivePlacement('run');
    document.querySelector('#presentation').textContent = width + ' px workspace. ' + (restricted && restricted.mode === 'maximized' ? 'Windows fill the available space. Wider placements are saved.' : 'Windows use their saved placements.') + ' The dock strip stays available.';
  }
  document.querySelectorAll('[data-width]').forEach(button => button.addEventListener('click', () => { preview.style.width = button.dataset.width === 'fit' ? '100%' : button.dataset.width + 'px'; }));
  document.querySelector('#theme').addEventListener('change', event => pudlSetTheme(event.target.value.toLowerCase()));
  document.querySelector('#policy').addEventListener('change', event => { if(event.target.checked) layer.setAttribute('data-win-narrow','maximized'); else layer.removeAttribute('data-win-narrow'); });
  document.addEventListener('click', event => { const opener=event.target.closest('[data-open]'); if(opener) pudlWindows.open(opener.dataset.open); if(event.target.id === 'run-again') document.querySelector('#run-status').textContent='This mockup starts no work.'; });
  new ResizeObserver(status).observe(layer); layer.addEventListener('pudl:windows-policy',status); layer.addEventListener('pudl:windows-change',status); status();
}
if(document.readyState === 'loading') document.addEventListener('DOMContentLoaded',setupMockup); else setupMockup();
