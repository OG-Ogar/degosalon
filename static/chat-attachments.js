(function () {
  var form = document.querySelector('[data-chat-composer]');
  if (!form) return;

  // The composer accepts one photo or clip, and voice notes on their own.
  // The server accepts the same caps; these only catch a mistake early.
  var MAX_IMAGE = 50 * 1024 * 1024;
  var MAX_VIDEO = 500 * 1024 * 1024;
  var MAX_VOICE = 8 * 1024 * 1024;
  var MAX_SECONDS = 120;
  var SLIDE_CANCEL = 140;
  var VIDEO_EXTENSIONS = /\.(mp4|webm|mov|m4v|3gp)$/i;
  var WAV_RATE = 16000;

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
          ? 'That video is larger than 500 MB. Choose a shorter clip.'
          : 'That photo is larger than 50 MB. Choose a smaller one.',
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

  // Prefer MP4/AAC everywhere it exists: iPhone Safari and modern Chrome can
  // both record it, and both platforms can play it back. Older Android Chrome
  // versions can only record WebM/Opus, which Safari refuses to play, so
  // onStopped() converts those recordings to WAV (also playable on both).
  function pickType() {
    var types = [
      'audio/mp4;codecs=mp4a.40.2',
      'audio/mp4',
      'audio/aac',
      'audio/webm;codecs=opus',
      'audio/webm',
      'audio/ogg;codecs=opus',
      'audio/ogg'
    ];
    for (var i = 0; i < types.length; i++) {
      if (MediaRecorder.isTypeSupported(types[i])) return types[i];
    }
    return '';
  }

  function isMp4(type) {
    return type.indexOf('mp4') !== -1 || type.indexOf('m4a') !== -1;
  }

  // Convert a WebM/Ogg recording to mono 16-bit WAV. WAV is the only audio
  // container every iPhone and Android browser plays without a plugin, so a
  // note recorded on an older Android is still audible on an iPhone.
  function toWav(blob) {
    return new Promise(function (resolve, reject) {
      var AudioContextClass = window.AudioContext || window.webkitAudioContext;
      if (!AudioContextClass || !window.FileReader) {
        reject(new Error('audio conversion is not available'));
        return;
      }

      var context = new AudioContextClass();
      var reader = new FileReader();

      reader.onerror = function () {
        context.close();
        reject(new Error('could not read the recording'));
      };

      reader.onload = function () {
        context.decodeAudioData(reader.result).then(function (buffer) {
          var frames = Math.max(1, Math.round(buffer.duration * WAV_RATE));
          var channels = [];
          var channel;
          for (channel = 0; channel < buffer.numberOfChannels; channel++) {
            channels.push(buffer.getChannelData(channel));
          }

          var pcm = new ArrayBuffer(44 + frames * 2);
          var view = new DataView(pcm);

          function text(offset, value) {
            for (var i = 0; i < value.length; i++) {
              view.setUint8(offset + i, value.charCodeAt(i));
            }
          }

          text(0, 'RIFF');
          view.setUint32(4, 36 + frames * 2, true);
          text(8, 'WAVE');
          text(12, 'fmt ');
          view.setUint32(16, 16, true);
          view.setUint16(20, 1, true);
          view.setUint16(22, 1, true);
          view.setUint32(24, WAV_RATE, true);
          view.setUint32(28, WAV_RATE * 2, true);
          view.setUint16(32, 2, true);
          view.setUint16(34, 16, true);
          text(36, 'data');
          view.setUint32(40, frames * 2, true);

          // Mix to mono and resample with linear interpolation. 16 kHz keeps a
          // two-minute note near 3.8 MB while staying clear for speech.
          var sourceRate = buffer.sampleRate;
          for (var i = 0; i < frames; i++) {
            var position = i * sourceRate / WAV_RATE;
            var left = Math.floor(position);
            var right = Math.min(left + 1, buffer.length - 1);
            var mix = position - left;
            var sample = 0;

            for (var c = 0; c < channels.length; c++) {
              sample += channels[c][left] * (1 - mix) + channels[c][right] * mix;
            }

            sample /= channels.length;
            sample = Math.max(-1, Math.min(1, sample));
            view.setInt16(44 + i * 2, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true);
          }

          context.close();
          resolve(new Blob([pcm], { type: 'audio/wav' }));
        }).catch(function () {
          context.close();
          reject(new Error('could not decode the recording'));
        });
      };

      reader.readAsArrayBuffer(blob);
    });
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

    if (isMp4(type)) {
      sendVoice(blob, type);
      return;
    }

    // Older Android browsers record WebM/Opus. Convert it so an iPhone can
    // play the note too, instead of silently sending a format Safari rejects.
    phase = 'sending';
    mic.disabled = true;
    status('Preparing voice note…', false);

    toWav(blob).then(function (wav) {
      if (wav.size > MAX_VOICE) {
        phase = 'idle';
        mic.disabled = false;
        status('That voice note is too long. Keep it under 2 minutes.', true);
        return;
      }
      sendVoice(wav, 'audio/wav');
    }).catch(function () {
      phase = 'idle';
      mic.disabled = false;
      status('This browser recorded a format other phones cannot play. Update your browser and try again.', true);
    });
  }

  function sendVoice(blob, type) {
    var ext = type.indexOf('mp4') !== -1
      ? 'm4a'
      : (type.indexOf('wav') !== -1 ? 'wav' : (type.indexOf('ogg') !== -1 ? 'ogg' : 'webm'));
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

  // A large clip can take several minutes. The browser's native form post has
  // no progress bar, so at least show that something is happening.
  form.addEventListener('submit', function () {
    if (photoInput.files && photoInput.files.length) {
      status('Uploading… this can take a few minutes for a large video.', false);
    }
  });

  refresh();
})();
