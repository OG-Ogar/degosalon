(function () {
  var form = document.querySelector('[data-chat-composer]');
  if (!form) return;

  // The composer accepts one photo or clip, and voice notes on their own.
  // The server accepts the same caps; these only catch a mistake early.
  var MAX_IMAGE = 20 * 1024 * 1024;
  var MAX_VIDEO = 50 * 1024 * 1024;
  var MAX_VOICE = 5 * 1024 * 1024;
  var MAX_SECONDS = 120;
  var SLIDE_CANCEL = 140;
  var VIDEO_EXTENSIONS = /\.(mp4|webm|mov|m4v|3gp)$/i;

  var photoInput = form.querySelector('[data-photo-input]');
  var preview = form.querySelector('[data-photo-preview]');
  var previewImg = form.querySelector('[data-photo-preview-img]');
  var previewVideo = form.querySelector('[data-photo-preview-video]');
  var previewBadge = form.querySelector('[data-photo-preview-badge]');
  var removeBtn = form.querySelector('[data-photo-remove]');
  var mic = form.querySelector('[data-mic]');
  var textarea = form.querySelector('textarea');
  var recBar = form.querySelector('[data-recording-bar]');
  var recTime = form.querySelector('[data-recording-time]');
  var recHint = recBar ? recBar.querySelector('.rec-hint') : null;
  var statusBox = form.parentNode.querySelector('[data-composer-status]');

  function status(message, isError) {
    if (!statusBox) return;
    if (!message) {
      statusBox.hidden = true;
      statusBox.textContent = '';
      statusBox.classList.remove('error');
      return;
    }
    statusBox.textContent = message;
    statusBox.hidden = false;
    statusBox.classList.toggle('error', !!isError);
  }

  // Show Send when there is something to send, otherwise show the mic.
  function refresh() {
    var hasText = textarea.value.trim() !== '';
    var hasFile = photoInput.files && photoInput.files.length > 0;
    form.classList.toggle('has-content', hasText || hasFile);
  }
  textarea.addEventListener('input', refresh);

  // ---------- Photo / video preview ----------
  var previewUrl = null;

  function isVideoFile(file) {
    if (/^video\//i.test(file.type)) return true;
    return !/^image\//i.test(file.type) && VIDEO_EXTENSIONS.test(file.name);
  }

  function resetPreview() {
    preview.hidden = true;
    form.classList.remove('has-attachment');
    previewImg.hidden = true;
    previewImg.removeAttribute('src');
    previewVideo.hidden = true;
    previewVideo.removeAttribute('src');
    previewBadge.hidden = true;
    if (previewUrl) {
      URL.revokeObjectURL(previewUrl);
      previewUrl = null;
    }
  }

  function clearAttachment() {
    photoInput.value = '';
    resetPreview();
    status('');
    refresh();
  }

  photoInput.addEventListener('change', function () {
    var file = photoInput.files && photoInput.files[0];
    if (!file) {
      resetPreview();
      refresh();
      return;
    }

    var video = isVideoFile(file);
    var limit = video ? MAX_VIDEO : MAX_IMAGE;
    if (file.size > limit) {
      clearAttachment();
      status(
        video
          ? 'That video is larger than 50 MB. Choose a shorter clip.'
          : 'That photo is larger than 20 MB. Choose a smaller one.',
        true
      );
      return;
    }

    status('');
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(file);

    if (video) {
      previewImg.hidden = true;
      previewImg.removeAttribute('src');
      // #t=0.1 asks the browser to draw a frame instead of an empty box.
      previewVideo.src = previewUrl + '#t=0.1';
      previewVideo.hidden = false;
      previewBadge.hidden = false;
    } else {
      previewVideo.hidden = true;
      previewVideo.removeAttribute('src');
      previewImg.src = previewUrl;
      previewImg.hidden = false;
      previewBadge.hidden = true;
    }

    preview.hidden = false;
    form.classList.add('has-attachment');
    refresh();
  });
  removeBtn.addEventListener('click', clearAttachment);

  // ---------- Voice note: hold to record, release to send ----------
  var canRecord = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia && window.MediaRecorder);
  if (!canRecord) {
    mic.hidden = true;
    refresh();
    return;
  }

  // idle -> starting -> recording -> stopping -> sending -> idle
  var phase = 'idle';
  var recorder = null;
  var stream = null;
  var chunks = [];
  var startedAt = 0;
  var timer = null;
  var releasedWhileStarting = false;
  var cancelWhenReady = false;

  function pickType() {
    var types = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg;codecs=opus', 'audio/ogg'];
    for (var i = 0; i < types.length; i++) {
      if (MediaRecorder.isTypeSupported(types[i])) return types[i];
    }
    return '';
  }

  function stopTracks() {
    if (stream) {
      stream.getTracks().forEach(function (track) { track.stop(); });
      stream = null;
    }
  }

  function tick() {
    var seconds = Math.floor((Date.now() - startedAt) / 1000);
    recTime.textContent = Math.floor(seconds / 60) + ':' + String(seconds % 60).padStart(2, '0');
    if (seconds >= MAX_SECONDS) stop(false);
  }

  function cleanup() {
    clearInterval(timer);
    timer = null;
    recBar.hidden = true;
    mic.classList.remove('recording');
    if (recHint) recHint.textContent = 'Release to send · slide away to cancel';
    stopTracks();
  }

  function start() {
    if (phase !== 'idle') return;
    phase = 'starting';
    releasedWhileStarting = false;
    cancelWhenReady = false;
    chunks = [];

    navigator.mediaDevices.getUserMedia({ audio: true }).then(function (captured) {
      if (releasedWhileStarting) {
        // The permission prompt took longer than the press. Nothing was
        // recorded, so explain instead of silently dropping the note.
        captured.getTracks().forEach(function (track) { track.stop(); });
        phase = 'idle';
        status('Microphone ready. Hold the mic and speak to record.', false);
        return;
      }

      stream = captured;

      try {
        var type = pickType();
        recorder = type ? new MediaRecorder(stream, { mimeType: type }) : new MediaRecorder(stream);
      } catch (err) {
        cleanup();
        phase = 'idle';
        status('This browser cannot record a voice note. Try attaching a video instead.', true);
        return;
      }

      recorder.ondataavailable = function (event) {
        if (event.data && event.data.size) chunks.push(event.data);
      };
      recorder.onstop = onStopped;
      recorder.onerror = function () {
        cleanup();
        phase = 'idle';
        status('Recording failed. Please try again.', true);
      };

      // A timeslice keeps data flowing, so a long note is not lost if the
      // page is backgrounded before the user releases the button.
      try {
        recorder.start(250);
      } catch (err) {
        cleanup();
        phase = 'idle';
        status('This browser cannot record a voice note. Try attaching a video instead.', true);
        return;
      }
      startedAt = Date.now();
      recTime.textContent = '0:00';
      recBar.hidden = false;
      mic.classList.add('recording');
      phase = 'recording';
      timer = setInterval(tick, 250);
    }).catch(function () {
      phase = 'idle';
      status('Microphone access was blocked. Allow the microphone in your browser settings to send voice notes.', true);
    });
  }

  function stop(cancel) {
    cancelWhenReady = cancel;

    if (phase === 'starting') {
      releasedWhileStarting = true;
      return;
    }

    if (phase !== 'recording') return;

    phase = 'stopping';

    if (recorder && recorder.state !== 'inactive') {
      recorder.stop();
    } else {
      cleanup();
      phase = 'idle';
    }
  }

  function onStopped() {
    var seconds = (Date.now() - startedAt) / 1000;
    var type = (recorder && recorder.mimeType) || 'audio/webm';
    cleanup();

    if (cancelWhenReady) {
      phase = 'idle';
      return;
    }

    if (seconds < 1 || !chunks.length) {
      phase = 'idle';
      status('Hold the mic a little longer to record a voice note.', true);
      return;
    }

    var blob = new Blob(chunks, { type: type });
    if (blob.size > MAX_VOICE) {
      phase = 'idle';
      status('That voice note is too long. Keep it under 2 minutes.', true);
      return;
    }

    sendVoice(blob, type);
  }

  function sendVoice(blob, type) {
    var ext = type.indexOf('mp4') !== -1 ? 'm4a' : (type.indexOf('ogg') !== -1 ? 'ogg' : 'webm');
    var data = new FormData(form);
    data.delete('photo');
    data.append('voice', blob, 'voice-note.' + ext);

    phase = 'sending';
    mic.disabled = true;
    status('Sending voice note…', false);

    fetch(form.action, {
      method: 'POST',
      body: data,
      credentials: 'same-origin',
      redirect: 'follow'
    }).then(function (response) {
      if (!response.ok) {
        throw new Error('server returned ' + response.status);
      }
      window.location.href = form.action;
    }).catch(function (error) {
      phase = 'idle';
      mic.disabled = false;
      status(
        'The voice note could not be sent (' + (error && error.message ? error.message : 'connection problem') + '). Please try again.',
        true
      );
    });
  }

  // Pointer events cover mouse, touch and pen. The distance is measured from
  // where the press started, which also works on browsers that report 0,0 for
  // the release point while a pointer is captured.
  var press = null;
  var slideCancelled = false;

  mic.addEventListener('pointerdown', function (event) {
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    event.preventDefault();
    press = { x: event.clientX, y: event.clientY, id: event.pointerId };
    slideCancelled = false;
    try { mic.setPointerCapture(event.pointerId); } catch (err) {}
    start();
  });

  mic.addEventListener('pointermove', function (event) {
    if (!press || event.pointerId !== press.id) return;
    var dx = event.clientX - press.x;
    var dy = event.clientY - press.y;
    slideCancelled = Math.sqrt(dx * dx + dy * dy) > SLIDE_CANCEL;
    if (recHint) {
      recHint.textContent = slideCancelled
        ? 'Release to cancel'
        : 'Release to send · slide away to cancel';
    }
  });

  function release(event) {
    if (!press || (event && event.pointerId !== undefined && event.pointerId !== press.id)) return;
    var cancel = slideCancelled;
    press = null;
    slideCancelled = false;
    stop(cancel);
  }

  mic.addEventListener('pointerup', release);
  mic.addEventListener('pointercancel', function (event) {
    if (!press) return;
    press = null;
    slideCancelled = false;
    stop(true);
  });
  // Capture can be lost (the OS may take over the gesture); never leave the
  // recorder running because the matching pointerup went somewhere else.
  window.addEventListener('pointerup', release);
  window.addEventListener('pointercancel', function () {
    if (!press) return;
    press = null;
    slideCancelled = false;
    stop(true);
  });
  mic.addEventListener('contextmenu', function (event) { event.preventDefault(); });

  // Keyboard users can hold Space or Enter on the focused mic button.
  mic.addEventListener('keydown', function (event) {
    if (event.repeat) return;
    if (event.key === ' ' || event.key === 'Enter') {
      event.preventDefault();
      start();
    }
  });
  mic.addEventListener('keyup', function (event) {
    if (event.key === ' ' || event.key === 'Enter') {
      event.preventDefault();
      stop(false);
    }
  });

  refresh();
})();
