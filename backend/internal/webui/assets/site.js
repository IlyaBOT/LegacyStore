(function () {
  function hasClass(node, name) {
    return (" " + node.className + " ").indexOf(" " + name + " ") !== -1;
  }

  function addClass(node, name) {
    if (!hasClass(node, name)) {
      node.className = node.className ? node.className + " " + name : name;
    }
  }

  function removeClass(node, name) {
    var pattern = new RegExp("(^|\\s)" + name + "(?=\\s|$)", "g");
    node.className = node.className.replace(pattern, " ").replace(/^\s+|\s+$/g, "");
  }

  function wireDangerForms() {
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

  function wireHistory() {
    var back = document.getElementById("historyBack");
    var forward = document.getElementById("historyForward");
    if (back) {
      back.onclick = function () { window.history.back(); };
    }
    if (forward) {
      forward.onclick = function () { window.history.forward(); };
    }
  }

  function wireCarousel() {
    var carousel = document.getElementById("homeCarousel");
    if (!carousel) {
      return;
    }

    var all = carousel.getElementsByTagName("*");
    var slides = [];
    var dots = [];
    var i;
    for (i = 0; i < all.length; i += 1) {
      if (hasClass(all[i], "home-carousel-slide")) {
        slides.push(all[i]);
      } else if (hasClass(all[i], "home-carousel-dot")) {
        dots.push(all[i]);
      }
    }
    if (slides.length < 1) {
      return;
    }

    var current = 0;
    var timer = null;

    function activate(index) {
      current = (index + slides.length) % slides.length;
      for (i = 0; i < slides.length; i += 1) {
        if (i === current) {
          addClass(slides[i], "is-active");
        } else {
          removeClass(slides[i], "is-active");
        }
      }
      for (i = 0; i < dots.length; i += 1) {
        if (i === current) {
          addClass(dots[i], "is-active");
        } else {
          removeClass(dots[i], "is-active");
        }
      }
    }

    function stop() {
      if (timer) {
        window.clearInterval(timer);
        timer = null;
      }
    }

    function start() {
      stop();
      if (slides.length > 1) {
        timer = window.setInterval(function () {
          activate(current + 1);
        }, 6000);
      }
    }

    for (i = 0; i < dots.length; i += 1) {
      (function (index) {
        dots[index].onclick = function () {
          activate(index);
          start();
        };
      }(i));
    }

    var previous = document.getElementById("carouselPrevious");
    var next = document.getElementById("carouselNext");
    if (previous) {
      previous.onclick = function () {
        activate(current - 1);
        start();
      };
    }
    if (next) {
      next.onclick = function () {
        activate(current + 1);
        start();
      };
    }

    carousel.onmouseover = stop;
    carousel.onmouseout = start;
    start();
  }

  function wireImageFallbacks() {
    var images = document.getElementsByTagName("img");
    var i;
    for (i = 0; i < images.length; i += 1) {
      if (hasClass(images[i], "app-icon-image")) {
        images[i].onerror = function () {
          addClass(this.parentNode, "is-broken");
        };
        images[i].onload = function () {
          removeClass(this.parentNode, "is-broken");
        };
        if (images[i].complete && typeof images[i].naturalWidth !== "undefined" && images[i].naturalWidth === 0) {
          addClass(images[i].parentNode, "is-broken");
        }
      }
    }
  }

  function wire() {
    wireDangerForms();
    wireHistory();
    wireImageFallbacks();
    wireCarousel();
  }

  if (document.addEventListener) {
    document.addEventListener("DOMContentLoaded", wire, false);
  } else if (window.attachEvent) {
    window.attachEvent("onload", wire);
  } else {
    window.onload = wire;
  }
}());
