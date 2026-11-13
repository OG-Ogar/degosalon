(function () {
  /* ---------------------------------------------------------------------
   * Composer: text, photo/video preview and a stable upload snapshot.
   * ------------------------------------------------------------------- */
  function initComposer() {
    var form = document.querySelector('[data-social-composer]');
    if (!form) return;

    var text = form.querySelector('[data-social-text]');
    var file = form.querySelector('[data-social-file]');
    var preview = form.querySelector('[data-social-preview]');
    var image = form.querySelector('[data-social-preview-image]');
    var video = form.querySelector('[data-social-preview-video]');
    var remove = form.querySelector('[data-social-remove]');
    var color = form.querySelector('[data-social-color]');
    var swatch = form.querySelector('[data-social-swatch]');
    var style = form.querySelector('[data-social-style]');
    var status = form.querySelector('[data-social-status]');
    var objectURL = '';

    /* Chrome re-checks a selected file's on-disk metadata when the form is
       submitted and aborts with ERR_UPLOAD_FILE_CHANGED if the original file
       moved, synced or was touched. Copying the bytes into memory as soon as
       the file is chosen means the upload no longer depends on that file
       reference. Large files stay on the normal path to avoid a memory spike. */
    var SNAPSHOT_LIMIT = 128 * 1024 * 1024;
    var snapshotReady = true;
    var snapshotPromise = null;
    var snapshotToken = 0;
    var preparingSubmit = false;

    function snapshotFile(selected) {
      var token = ++snapshotToken;
      snapshotReady = true;
      snapshotPromise = null;
      if (!selected || !window.DataTransfer || typeof selected.arrayBuffer !== 'function' || selected.size > SNAPSHOT_LIMIT) {
        return;
      }
      snapshotReady = false;
      snapshotPromise = selected.arrayBuffer().then(function (buffer) {
        /* A newer selection (or Remove) won the race. */
        if (token !== snapshotToken || !file.files || file.files[0] !== selected) return selected;
        var stable = new File([buffer], selected.name, {
          type: selected.type || 'application/octet-stream',
          lastModified: selected.lastModified
        });
        var transfer = new DataTransfer();
        transfer.items.add(stable);
        file.files = transfer.files;
        snapshotReady = true;
        return stable;
      }).catch(function () {
        /* Fall back to the original file if the browser cannot read it. */
        if (token === snapshotToken && file.files && file.files[0] === selected) snapshotReady = true;
        return selected;
      });
    }

    function clearPreview(clearSelection) {
      if (clearSelection && file) file.value = '';
      snapshotToken++;
      snapshotReady = true;
      snapshotPromise = null;
      preparingSubmit = false;
      preview.hidden = true;
      image.hidden = true;
      image.removeAttribute('src');
      video.pause();
      video.hidden = true;
      video.removeAttribute('src');
      if (objectURL) URL.revokeObjectURL(objectURL);
      objectURL = '';
    }

    function updateTextStyle() {
      text.style.color = '#172033';
      text.style.backgroundColor = color.value;
      if (swatch) swatch.style.backgroundColor = color.value;
      text.style.fontWeight = style.value === 'bold' ? '800' : '';
      text.style.fontStyle = style.value === 'italic' ? 'italic' : '';
    }

    form.querySelector('.social-composer-top').addEventListener('click', function (event) {
      if (event.target === event.currentTarget) text.focus();
    });
    color.addEventListener('change', updateTextStyle);
    style.addEventListener('change', updateTextStyle);
    updateTextStyle();

    file.addEventListener('change', function () {
      var selected = file.files && file.files[0];
      clearPreview(false);
      if (!selected) return;
      if (selected.size > 500 * 1024 * 1024) {
        file.value = '';
        status.textContent = 'This file exceeds the 500 MB app limit. Your storage provider may allow less.';
        status.hidden = false;
        return;
      }
      status.hidden = true;
      objectURL = URL.createObjectURL(selected);
      preview.hidden = false;
      if (/^video\//i.test(selected.type) || /\.(mp4|webm|mov|m4v|3gp)$/i.test(selected.name)) {
        video.src = objectURL;
        video.hidden = false;
      } else {
        image.src = objectURL;
        image.hidden = false;
      }
      snapshotFile(selected);
    });

    remove.addEventListener('click', function () { clearPreview(true); });

    form.addEventListener('submit', function (event) {
      var hasFile = file.files && file.files.length > 0;
      if (!text.value.trim() && !hasFile) {
        event.preventDefault();
        status.textContent = 'Write something or add a photo/video before posting.';
        status.hidden = false;
        text.focus();
        return;
      }
      if (!snapshotReady && snapshotPromise) {
        event.preventDefault();
        if (preparingSubmit) return;
        preparingSubmit = true;
        var token = snapshotToken;
        status.textContent = 'Preparing your photo or video…';
        status.hidden = false;
        snapshotPromise.then(function () {
          preparingSubmit = false;
          if (token !== snapshotToken || !snapshotReady) return;
          status.hidden = true;
          if (typeof form.requestSubmit === 'function') form.requestSubmit();
          else form.submit();
        });
        return;
      }
    });

    window.addEventListener('pagehide', function () {
      if (objectURL) URL.revokeObjectURL(objectURL);
    });
  }

  initComposer();
})();