(function () {
  var form = document.querySelector('[data-chat-composer]');
  if (!form) return;

  var MAX_PHOTO = 5 * 1024 * 1024;
  var MAX_SECONDS = 120;

  var photoInput = form.querySelector('[data-photo-input]');
  var preview = form.querySelector('[data-photo-preview]');
  var previewImg = form.querySelector('[data-photo-preview-img]');
  var removeBtn = form.querySelector('[data-photo-remove]');
  var mic = form.querySelector('[data-mic]');
  var textarea = form.querySelector('textarea');
  var recBar = form.querySelector('[data-recording-bar]');
  var recTime = form.querySelector('[data-recording-time]');

  // Show Send when there is something to send, otherwise show the mic.
  function refresh() {
    var hasText = textarea.value.trim() !== '';
    var hasPhoto = photoInput.files && photoInput.files.length > 0;
    form.classList.toggle('has-content', hasText || hasPhoto);
  }
  textarea.addEventListener('input', refresh);

  // ---------- Photo preview ----------
  var previewUrl = null;
  function clearPhoto() {
    photoInput.value = '';
    preview.hidden = true;
    if (previewUrl) { URL.revokeObjectURL(previewUrl); previewUrl = null; }
    previewImg.removeAttribute('src');
    refresh();
  }
  photoInput.addEventListener('change', function () {
    var file = photoInput.files && photoInput.files[0];
    if (!file) { clearPhoto(); return; }
    if (file.size > MAX_PHOTO) {
      alert('That photo is larger than 5 MB. Choose a smaller one.');
      clearPhoto();
      return;
    }
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(file);
    previewImg.src = previewUrl;
    preview.hidden = false;
    refresh();
  });
  removeBtn.addEventListener('click', clearPhoto);

  // ---------- Voice note: hold to record, release to send ----------
  var canRecord = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia && window.MediaRecorder);
  if (!canRecord) { mic.hidden = true; refresh(); return; }

  var recorder = null, stream = null, chunks = [], startedAt = 0, timer = null;
  var cancelled = false, active = false;

  function pickType() {
    var types = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg;codecs=opus'];
    for (var i = 0; i < types.length; i++) {
      if (MediaRecorder.isTypeSupported(types[i])) return types[i];
    }
    return '';
  }

  function tick() {
    var s = Math.floor((Date.now() - startedAt) / 1000);
    recTime.textContent = Math.floor(s / 60) + ':' + String(s % 60).padStart(2, '0');
    if (s >= MAX_SECONDS) stop(false);
  }

  function cleanup() {
    clearInterval(timer);
    recBar.hidden = true;
    mic.classList.remove('recording');
    if (stream) { stream.getTracks().forEach(function (t) { t.stop(); }); stream = null; }
    active = false;
  }

  function start() {
    if (active) return;
    active = true;
    cancelled = false;
    navigator.mediaDevices.getUserMedia({ audio: true }).then(function (s) {
      if (!active) { s.getTracks().forEach(function (t) { t.stop(); }); return; }
      stream = s;
      chunks = [];
      var type = pickType();
      recorder = type ? new MediaRecorder(s, { mimeType: type }) : new MediaRecorder(s);
      recorder.ondataavailable = function (e) { if (e.data && e.data.size) chunks.push(e.data); };
      recorder.onstop = onStopped;
      recorder.start();
      startedAt = Date.now();
      recTime.textContent = '0:00';
      recBar.hidden = false;
      mic.classList.add('recording');
      timer = setInterval(tick, 250);
    }).catch(function () {
      active = false;
      alert('Microphone access was blocked. Allow the microphone in your browser settings to send voice notes.');
    });
  }

  function stop(cancel) {
    if (!active) return;
    cancelled = cancel;
    if (recorder && recorder.state !== 'inactive') {
      recorder.stop();
    } else {
      cleanup();
    }
  }

  function onStopped() {
    var seconds = (Date.now() - startedAt) / 1000;
    var type = (recorder && recorder.mimeType) || 'audio/webm';
    cleanup();
    if (cancelled || seconds < 1 || !chunks.length) return;

    var blob = new Blob(chunks, { type: type });
    if (blob.size > MAX_PHOTO) { alert('That voice note is too long. Keep it under 2 minutes.'); return; }

    var ext = type.indexOf('mp4') !== -1 ? 'm4a' : (type.indexOf('ogg') !== -1 ? 'ogg' : 'webm');
    var data = new FormData(form);
    data.delete('photo');
    data.append('voice', blob, 'voice-note.' + ext);

    mic.disabled = true;
    fetch(form.action, { method: 'POST', body: data, credentials: 'same-origin' })
      .then(function (res) {
        if (res.ok) {
          window.location.href = form.action;
        } else {
          mic.disabled = false;
          alert('The voice note could not be sent. Please try again.');
        }
      })
      .catch(function () {
        mic.disabled = false;
        alert('Could not send the voice note. Check your connection and try again.');
      });
  }

  // Pointer events cover mouse, touch and pen.
  mic.addEventListener('pointerdown', function (e) {
    e.preventDefault();
    try { mic.setPointerCapture(e.pointerId); } catch (err) {}
    start();
  });
  mic.addEventListener('pointerup', function (e) {
    // Sliding well away from the button before release cancels the recording.
    var r = mic.getBoundingClientRect();
    var away = Math.abs(e.clientX - (r.left + r.width / 2)) > 120 || Math.abs(e.clientY - (r.top + r.height / 2)) > 120;
    stop(away);
  });
  mic.addEventListener('pointercancel', function () { stop(true); });
  mic.addEventListener('contextmenu', function (e) { e.preventDefault(); });

  refresh();
})();