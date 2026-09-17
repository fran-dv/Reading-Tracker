// Live readback for time typed the way it is said ("1h30", "1:30", "90").
// It mirrors parseMinutes in internal/web/duration.go, which is the parser
// that counts; this only shows how the server will read the field.
(function () {
  function minutes(s) {
    s = (s || "").trim().toLowerCase().replace(",", ".");
    let m;
    if ((m = s.match(/^(\d+):(\d{1,2})$/))) return +m[2] < 60 ? +m[1] * 60 + +m[2] : NaN;
    if ((m = s.match(/^(\d+(?:\.\d+)?)\s*h(?:ours?)?\s*(?:(\d+)\s*(?:m|min|mins|minutes?)?)?$/))) {
      return Math.round(+m[1] * 60) + (m[2] ? +m[2] : 0);
    }
    if ((m = s.match(/^(\d+)\s*(?:m|min|mins|minutes?)?$/))) return +m[1];
    return NaN;
  }

  // readDuration returns "= 1 h 30 min", a hint when it can't be read, or ""
  // for an empty field.
  window.readDuration = function (s) {
    if (!(s || "").trim()) return "";
    const n = minutes(s);
    if (isNaN(n)) return "Try 1h30, 1:30 or 90";
    const h = Math.floor(n / 60), r = n % 60;
    return "= " + (h ? h + " h " + String(r).padStart(2, "0") + " min" : r + " min");
  };
})();
