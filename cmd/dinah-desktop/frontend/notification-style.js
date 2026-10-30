/* Semantic notification classes use PUDL's status shapes and the brand colours. */
(function () {
  'use strict';
  var variants = { info: '', success: 'positive', warning: 'warn', error: 'danger' };
  window.dinahNotificationStyle = function (card, severity) {
    if (!Object.prototype.hasOwnProperty.call(variants, severity)) severity = 'info';
    card.classList.remove('positive', 'warn', 'danger', 'notification-info', 'notification-success', 'notification-warning', 'notification-error');
    card.classList.add('notification-' + severity);
    if (variants[severity]) card.classList.add(variants[severity]);
    card.dataset.severity = severity;
  };
})();
