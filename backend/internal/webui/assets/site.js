(function () {
  function hasClass(node, name) {
    return (" " + node.className + " ").indexOf(" " + name + " ") !== -1;
  }
  function wire() {
    var forms = document.getElementsByTagName("form");
    var i;
    for (i = 0; i < forms.length; i += 1) {
      if (hasClass(forms[i], "confirm-danger")) {
        forms[i].onsubmit = function () {
          return window.confirm("Подтвердить опасное действие?");
        };
      }
    }
  }
  if (document.addEventListener) {
    document.addEventListener("DOMContentLoaded", wire, false);
  } else if (window.attachEvent) {
    window.attachEvent("onload", wire);
  } else {
    window.onload = wire;
  }
}());