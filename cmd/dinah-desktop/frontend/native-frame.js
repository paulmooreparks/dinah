
/* Dinah host shortcuts use the SDK's existing typed host-command bridge. */
document.addEventListener('keydown', function (event) {
  if (event.key !== 'F11' || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || event.defaultPrevented) return;
  event.preventDefault(); event.stopPropagation();
  if(event.repeat || event.isComposing) return;
  window.dinah.hostCommand('toggle-fullscreen').catch(function () {});
}, true);
