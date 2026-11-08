(function () {
  var dialog = document.querySelector('[data-message-dialog]');
  if (!dialog) return;

  var menu = dialog.querySelector('[data-message-menu]');
  var editForm = dialog.querySelector('[data-message-edit-form]');
  var deleteForm = dialog.querySelector('[data-message-delete-form]');
  var textarea = dialog.querySelector('[data-message-edit-text]');
  var hint = dialog.querySelector('[data-message-edit-hint]');
  var editOpen = dialog.querySelector('[data-message-edit-open]');
  var deleteOpen = dialog.querySelector('[data-message-delete-open]');
  var editBack = dialog.querySelector('[data-message-edit-back]');
  var deleteBack = dialog.querySelector('[data-message-delete-back]');
  var closeButton = dialog.querySelector('[data-message-dialog-close]');

  var LONG_PRESS_MS = 500;
  var MOVE_TOLERANCE = 12;

  var current = null;
  var timer = null;
  var pointerId = null;
  var startX = 0;
  var startY = 0;
  var suppressClick = false;
  var suppressTimer = null;

  function showMenu() {
    menu.hidden = false;
    editForm.hidden = true;
    deleteForm.hidden = true;
  }

  function showEdit() {
    if (!current) return;
    menu.hidden = true;
    deleteForm.hidden = true;
    editForm.hidden = false;

    textarea.value = current.dataset.messageText || '';

    var hasMedia = current.dataset.messageHasMedia === 'true';
    if (hint) {
      hint.hidden = !hasMedia;
      hint.textContent = hasMedia
        ? 'This message includes a photo, video or voice note. You can change the text caption here, or delete the whole message.'
        : '';
    }

    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);
  }

  function showDelete() {
    if (!current) return;
    menu.hidden = true;
    editForm.hidden = true;
    deleteForm.hidden = false;
  }

  function openFor(bubble) {
    current = bubble;
    editForm.setAttribute('action', bubble.dataset.messageEditUrl || '');
    deleteForm.setAttribute('action', bubble.dataset.messageDeleteUrl || '');
    showMenu();

    if (!dialog.open) {
      if (typeof dialog.showModal === 'function') {
        dialog.showModal();
      } else {
        dialog.setAttribute('open', '');
      }
    }

    document.body.classList.add('message-actions-open');
  }

  function closeDialog() {
    if (typeof dialog.close === 'function' && dialog.open) {
      dialog.close();
    } else {
      dialog.removeAttribute('open');
    }
    document.body.classList.remove('message-actions-open');
  }

  function clearPress() {
    if (timer !== null) {
      window.clearTimeout(timer);
      timer = null;
    }
    pointerId = null;
  }

  function cancelPress() {
    clearPress();
  }

  function scheduleSuppressReset() {
    if (suppressTimer !== null) {
      window.clearTimeout(suppressTimer);
    }
    suppressTimer = window.setTimeout(function () {
      suppressClick = false;
      suppressTimer = null;
    }, 500);
  }

  document.addEventListener('pointerdown', function (event) {
    if (event.button !== 0) return;
    if (!event.target || typeof event.target.closest !== 'function') return;

    var bubble = event.target.closest('.bubble[data-message-own]');
    if (!bubble) return;
    if (event.target.closest('[data-message-menu-trigger]')) return;

    clearPress();
    pointerId = event.pointerId;
    startX = event.clientX;
    startY = event.clientY;

    timer = window.setTimeout(function () {
      timer = null;
      suppressClick = true;
      if (window.getSelection) {
        window.getSelection().removeAllRanges();
      }
      openFor(bubble);
    }, LONG_PRESS_MS);
  }, { passive: true });

  document.addEventListener('pointermove', function (event) {
    if (pointerId === null || event.pointerId !== pointerId) return;
    if (Math.abs(event.clientX - startX) > MOVE_TOLERANCE ||
        Math.abs(event.clientY - startY) > MOVE_TOLERANCE) {
      cancelPress();
    }
  }, { passive: true });

  document.addEventListener('pointerup', function (event) {
    if (pointerId !== null && event.pointerId === pointerId) {
      clearPress();
      if (suppressClick) {
        scheduleSuppressReset();
      }
    }
  }, { passive: true });

  document.addEventListener('pointercancel', cancelPress, { passive: true });

  document.addEventListener('click', function (event) {
    if (!suppressClick) return;
    event.preventDefault();
    event.stopPropagation();
    scheduleSuppressReset();
  }, true);

  document.addEventListener('contextmenu', function (event) {
    if (!event.target || typeof event.target.closest !== 'function') return;
    var bubble = event.target.closest('.bubble[data-message-own]');
    if (!bubble) return;
    event.preventDefault();
    openFor(bubble);
  });

  document.addEventListener('click', function (event) {
    if (!event.target || typeof event.target.closest !== 'function') return;
    var trigger = event.target.closest('[data-message-menu-trigger]');
    if (!trigger) return;
    var bubble = trigger.closest('.bubble[data-message-own]');
    if (!bubble) return;
    event.preventDefault();
    openFor(bubble);
  });

  if (editOpen) editOpen.addEventListener('click', showEdit);
  if (deleteOpen) deleteOpen.addEventListener('click', showDelete);
  if (editBack) editBack.addEventListener('click', showMenu);
  if (deleteBack) deleteBack.addEventListener('click', showMenu);
  if (closeButton) closeButton.addEventListener('click', closeDialog);

  dialog.addEventListener('click', function (event) {
    if (event.target === dialog) {
      closeDialog();
    }
  });

  dialog.addEventListener('close', function () {
    document.body.classList.remove('message-actions-open');
    current = null;
  });

  [editForm, deleteForm].forEach(function (form) {
    form.addEventListener('submit', function () {
      var submit = form.querySelector('button[type="submit"]');
      if (!submit) return;
      submit.disabled = true;
      submit.textContent = form === deleteForm ? 'Deleting…' : 'Saving…';
    });
  });
})();
