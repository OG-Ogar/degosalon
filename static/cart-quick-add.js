(function () {
  var forms = document.querySelectorAll('form[data-quick-add]');
  if (!forms.length) return;

  var badge = document.querySelector('[data-cart-count]');

  function toast(msg) {
    var t = document.createElement('div');
    t.className = 'quick-add-toast';
    t.setAttribute('role', 'status');
    t.textContent = msg;
    document.body.appendChild(t);
    setTimeout(function () { t.remove(); }, 2500);
  }

  function setBadge(n) {
    if (!badge) return;
    badge.textContent = n;
    badge.hidden = !(n > 0);
  }

  function stockEl(id) {
    return document.querySelector('[data-stock-for="' + id + '"] [data-stock-count]');
  }

  forms.forEach(function (form) {
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var btn = form.querySelector('button[type="submit"]');
      if (btn.disabled) return;

      var id = form.getAttribute('data-product-id');
      var stock = stockEl(id);
      var before = stock ? parseInt(stock.textContent, 10) : null;
      var badgeBefore = badge && !badge.hidden ? parseInt(badge.textContent, 10) || 0 : 0;

      // Instant update
      if (stock && before > 0) stock.textContent = before - 1;
      setBadge(badgeBefore + 1);
      btn.disabled = true;
      var label = btn.textContent;
      btn.textContent = 'Added';

      function rollback(msg) {
        if (stock && before !== null) stock.textContent = before;
        setBadge(badgeBefore);
        btn.textContent = label;
        btn.disabled = false;
        toast(msg);
      }

      fetch(form.action, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          'X-Quick-Add': '1',
          'Accept': 'application/json'
        },
        body: new URLSearchParams(new FormData(form)).toString()
      })
        .then(function (res) {
          return res.json().then(function (data) { return { res: res, data: data }; });
        })
        .then(function (r) {
          if (!r.res.ok || !r.data.ok) {
            rollback(r.data.message || 'Could not add to cart.');
            return;
          }
          setBadge(r.data.cartCount);
          toast('Added to cart');
          var left = stock ? parseInt(stock.textContent, 10) : 1;
          if (left <= 0) {
            btn.textContent = 'Out of stock';
          } else {
            btn.textContent = label;
            btn.disabled = false;
          }
        })
        .catch(function () { rollback('Network problem. Try again.'); });
    });
  });
})();