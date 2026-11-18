(function () {
  var KEY = 'sdmp_token';

  function token() {
    try { return localStorage.getItem(KEY) || ''; } catch (e) { return ''; }
  }
  function onLoginPage() {
    return location.pathname === '/login' || location.pathname.indexOf('/login/') === 0;
  }
  function loginURL() {
    var next = location.pathname + location.search + location.hash;
    return '/login/?next=' + encodeURIComponent(next);
  }
  function goLogin() {
    if (onLoginPage()) return;
    try { localStorage.removeItem(KEY); } catch (e) {}
    location.replace(loginURL());
  }
  function needsAuth(url) {
    if (!url || url.indexOf('/rest/') === -1) return false;
    if (url.indexOf('/rest/auth/login') !== -1) return false;
    if (url.indexOf('/rest/skorozvon/events') !== -1) return false;
    if (url.indexOf('/rest/crmcalls/events') !== -1) return false;
    if (url.indexOf('/rest/recordings/') !== -1) return false;
    if (/\/rest\/[^/]+\/[^/]+\//.test(url)) return false;
    return true;
  }

  if (!onLoginPage() && !token()) goLogin();

  var orig = window.fetch;
  window.fetch = function (input, init) {
    init = init ? Object.assign({}, init) : {};
    var url = typeof input === 'string' ? input : (input && input.url) || '';
    if (needsAuth(url) && !token()) {
      goLogin();
      return new Promise(function () {});
    }
    if (needsAuth(url)) {
      var headers = new Headers(init.headers || (input && input.headers) || undefined);
      if (!headers.has('Authorization')) headers.set('Authorization', 'Bearer ' + token());
      init.headers = headers;
    }
    return orig.call(this, input, init).then(function (res) {
      if (res.status === 401 && needsAuth(url)) {
        goLogin();
        return new Promise(function () {});
      }
      return res;
    });
  };
})();
