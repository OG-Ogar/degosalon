(function () {
  var form = document.querySelector('[data-social-composer]');
  if (!form) return;

  var text = form.querySelector('[data-social-text]');
  var file = form.querySelector('[data-social-file]');
  var preview = form.querySelector('[data-social-preview]');
  var image = form.querySelector('[data-social-preview-image]');
  var video = form.querySelector('[data-social-preview-video]');
  var remove = form.querySelector('[data-social-remove]');
  var color = form.querySelector('[data-social-color]');
  var style = form.querySelector('[data-social-style]');
  var status = form.querySelector('[data-social-status]');
  var objectURL = '';

  function clearPreview(clearSelection) {
    if (clearSelection) file.value = '';
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
    text.style.color = color.value;
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
  });
  remove.addEventListener('click', function () { clearPreview(true); });
  form.addEventListener('submit', function (event) {
    if (!text.value.trim() && (!file.files || file.files.length === 0)) {
      event.preventDefault();
      status.textContent = 'Write something or add a photo/video before posting.';
      status.hidden = false;
      text.focus();
    }
  });
  window.addEventListener('pagehide', function () {
    if (objectURL) URL.revokeObjectURL(objectURL);
  });
})();
