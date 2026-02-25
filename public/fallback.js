// Safety net: if the app module fails to load (e.g., CDN unreachable),
// show an error instead of an infinite loading spinner.
setTimeout(function () {
  var el = document.getElementById('loading-screen');
  if (el && !el.classList.contains('is-hidden')) {
    el.innerHTML =
      '<p style="text-align:center;padding:2rem;color:#f14668;">' +
      'Failed to load application. Check network connectivity and refresh.</p>';
  }
}, 10000);
