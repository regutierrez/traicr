// Applies the saved theme before the first paint. Loaded synchronously because
// the Content Security Policy forbids inline scripts.
(function () {
  var themes = ["ledger", "console", "blueprint"];
  var theme = "ledger";
  try {
    var saved = window.localStorage.getItem("traicr:theme");
    if (saved && themes.indexOf(saved) !== -1) theme = saved;
  } catch (error) {
    // Storage may be unavailable in private browsing; keep the default theme.
  }
  document.documentElement.setAttribute("data-theme", theme);
})();
