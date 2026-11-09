/* The page's way to the services a product built from this source adds
   beside the App's own methods (services.go, docs/design/product-
   applets.md). dinahServices.call sends a method's arguments as a JSON
   array and gives its answer; dinahServices.object(name) is the service as
   an object whose methods are its methods, so a product's page files call
   `svc.Account()` or `svc.Period(from, to)` as they would call the App. */
(function () {
  'use strict';
  function app() { return window.dinahHelpers.app(); }
  function call(service, method, args) {
    var a = app();
    if (!a || typeof a.CallService !== 'function') return Promise.reject(new Error('This page has no services.'));
    return Promise.resolve(a.CallService(service, method, JSON.stringify(args == null ? [] : Array.from(args)))).then(function (text) {
      return text === '' || text == null ? null : JSON.parse(text);
    });
  }
  function object(service) {
    var cache = {};
    return new Proxy(cache, { get: function (target, method) {
      if (typeof method !== 'string') return undefined;
      if (method === 'then') return undefined;
      if (!target[method]) target[method] = function () { return call(service, method, arguments); };
      return target[method];
    } });
  }
  window.dinahServices = { call: call, object: object };
})();
