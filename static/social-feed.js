(function () {
  var SOUND_PREF_KEY = 'og-social-sound';

  function readSoundPreference() {
    try {
      return window.sessionStorage.getItem(SOUND_PREF_KEY) !== 'off';
    } catch (error) {
      return true;
    }
  }

  function saveSoundPreference(on) {
    try {
      window.sessionStorage.setItem(SOUND_PREF_KEY, on ? 'on' : 'off');
    } catch (error) {
      /* Private browsing may block storage; the page still works. */
    }
  }

  /* ---------------------------------------------------------------------
   * Composer: optional soundtrack for a selected photo or video.
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
    var soundToggle = form.querySelector('[data-sound-toggle]');
    var soundPicker = form.querySelector('[data-sound-picker]');
    var soundClose = form.querySelector('[data-sound-close]');
    var soundChip = form.querySelector('[data-sound-chip]');
    var soundChipName = form.querySelector('[data-sound-chip-name]');
    var soundRemove = form.querySelector('[data-sound-remove]');
    var soundEmpty = form.querySelector('[data-sound-empty]');
    var soundOptions = Array.prototype.slice.call(form.querySelectorAll('[data-sound-option]'));
    var soundPreview = new Audio();
    var previewTrack = null;
    var objectURL = '';

    function selectedSound() {
      for (var i = 0; i < soundOptions.length; i++) {
        if (soundOptions[i].checked) return soundOptions[i];
      }
      return null;
    }

    function stopSoundPreview() {
      if (!previewTrack) return;
      soundPreview.pause();
      soundPreview.currentTime = 0;
      previewTrack.textContent = '▶';
      previewTrack.setAttribute('aria-label', previewTrack.getAttribute('aria-label').replace(/^Pause /, 'Preview '));
      previewTrack = null;
    }

    function updateSoundLabel() {
      var option = selectedSound();
      if (option) {
        soundToggle.textContent = '♪ Change sound';
        soundChip.hidden = false;
        soundChipName.textContent = option.closest('.social-sound-option').querySelector('strong').textContent;
        if (soundEmpty) soundEmpty.hidden = true;
      } else {
        soundToggle.textContent = '♪ Add sound';
        soundChip.hidden = true;
        soundChipName.textContent = '';
        if (soundEmpty) soundEmpty.hidden = false;
      }
    }

    function clearSound() {
      stopSoundPreview();
      for (var i = 0; i < soundOptions.length; i++) soundOptions[i].checked = false;
      updateSoundLabel();
    }

    function closeSoundPicker() {
      soundPicker.hidden = true;
      soundToggle.setAttribute('aria-expanded', 'false');
      stopSoundPreview();
    }

    function openSoundPicker() {
      soundPicker.hidden = false;
      soundToggle.setAttribute('aria-expanded', 'true');
      var first = soundOptions[0];
      if (first) first.focus();
    }

    function clearPreview(clearSelection, clearTrack) {
      if (clearSelection && file) file.value = '';
      preview.hidden = true;
      image.hidden = true;
      image.removeAttribute('src');
      video.pause();
      video.hidden = true;
      video.removeAttribute('src');
      if (objectURL) URL.revokeObjectURL(objectURL);
      objectURL = '';
      if (soundToggle) soundToggle.hidden = true;
      closeSoundPicker();
      if (clearTrack) clearSound();
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
      clearPreview(false, false);
      if (!selected) {
        clearSound();
        return;
      }
      if (selected.size > 500 * 1024 * 1024) {
        file.value = '';
        clearSound();
        status.textContent = 'This file exceeds the 500 MB app limit. Your storage provider may allow less.';
        status.hidden = false;
        return;
      }
      status.hidden = true;
      objectURL = URL.createObjectURL(selected);
      preview.hidden = false;
      soundToggle.hidden = false;
      if (/^video\//i.test(selected.type) || /\.(mp4|webm|mov|m4v|3gp)$/i.test(selected.name)) {
        video.src = objectURL;
        video.hidden = false;
      } else {
        image.src = objectURL;
        image.hidden = false;
      }
    });

    remove.addEventListener('click', function () { clearPreview(true, true); });

    soundToggle.addEventListener('click', function () {
      if (soundPicker.hidden) openSoundPicker();
      else closeSoundPicker();
    });
    soundClose.addEventListener('click', function () {
      closeSoundPicker();
      soundToggle.focus();
    });

    soundOptions.forEach(function (option) {
      option.addEventListener('change', function () {
        stopSoundPreview();
        updateSoundLabel();
      });
    });

    form.querySelectorAll('[data-sound-preview]').forEach(function (button) {
      function togglePreview(event) {
        event.preventDefault();
        event.stopPropagation();
        var src = button.getAttribute('data-src');
        if (previewTrack === button) {
          stopSoundPreview();
          return;
        }
        stopSoundPreview();
        previewTrack = button;
        soundPreview.src = src;
        soundPreview.loop = true;
        soundPreview.volume = 0.55;
        button.textContent = '❚❚';
        button.setAttribute('aria-label', button.getAttribute('aria-label').replace(/^Preview /, 'Pause '));
        var playing = soundPreview.play();
        if (playing && playing.catch) {
          playing.catch(function () {
            stopSoundPreview();
            status.textContent = 'Tap the track again to hear a preview.';
            status.hidden = false;
          });
        }
      }
      button.addEventListener('click', togglePreview);
      button.addEventListener('keydown', function (event) {
        if (event.key === 'Enter' || event.key === ' ') togglePreview(event);
      });
    });

    soundRemove.addEventListener('click', function () {
      clearSound();
      soundToggle.focus();
    });

    form.addEventListener('submit', function (event) {
      var hasFile = file.files && file.files.length > 0;
      if (!text.value.trim() && !hasFile) {
        event.preventDefault();
        status.textContent = 'Write something or add a photo/video before posting.';
        status.hidden = false;
        text.focus();
        return;
      }
      if (selectedSound() && !hasFile) {
        event.preventDefault();
        status.textContent = 'Add a photo or video before choosing a soundtrack.';
        status.hidden = false;
        return;
      }
      stopSoundPreview();
    });

    window.addEventListener('pagehide', function () {
      stopSoundPreview();
      if (objectURL) URL.revokeObjectURL(objectURL);
    });
  }

  /* ---------------------------------------------------------------------
   * Home feed: play a post's soundtrack while it is on screen, and stop it
   * as soon as the visitor scrolls past so the next post can take over.
   * ------------------------------------------------------------------- */
  function initFeedSound() {
    var cards = Array.prototype.slice.call(document.querySelectorAll('.social-post-card[data-post-music]'));
    if (!cards.length) return;

    var soundOn = readSoundPreference();
    var activeCard = null;

    var posts = cards.map(function (card) {
      var audio = card.querySelector('[data-post-audio]');
      return {
        card: card,
        audio: audio,
        button: card.querySelector('[data-post-sound-toggle]'),
        icon: card.querySelector('[data-post-sound-icon]'),
        label: card.querySelector('[data-post-sound-label]'),
        video: card.querySelector('video.social-post-media'),
        name: card.querySelector('[data-post-sound-label]') ? card.querySelector('[data-post-sound-label]').textContent : 'Soundtrack',
        blocked: false
      };
    });

    function postFor(card) {
      for (var i = 0; i < posts.length; i++) {
        if (posts[i].card === card) return posts[i];
      }
      return null;
    }

    function refreshButton(post) {
      if (!post || !post.button) return;
      if (!soundOn) {
        post.icon.textContent = '♪';
        post.label.textContent = post.name + ' · sound off';
        post.button.setAttribute('aria-pressed', 'false');
        post.button.setAttribute('aria-label', 'Soundtrack ' + post.name + '. Tap to turn sound on.');
        return;
      }
      if (post.blocked) {
        post.icon.textContent = '🔈';
        post.label.textContent = 'Tap to hear ' + post.name;
        post.button.setAttribute('aria-pressed', 'true');
        post.button.setAttribute('aria-label', 'Soundtrack ' + post.name + '. Tap to play it.');
        return;
      }
      post.icon.textContent = '♪';
      post.label.textContent = post.name;
      post.button.setAttribute('aria-pressed', 'true');
      post.button.setAttribute('aria-label', 'Soundtrack ' + post.name + '. Tap to turn sound off.');
    }

    function refreshAllButtons() {
      posts.forEach(refreshButton);
    }

    function pausePost(post, reset) {
      if (!post) return;
      post.audio.pause();
      if (reset) {
        try { post.audio.currentTime = 0; } catch (error) { /* not loaded yet */ }
      }
    }

    function playPost(post) {
      if (!post || !soundOn) return;
      post.audio.volume = 0.65;
      var playing = post.audio.play();
      post.blocked = false;
      if (playing && playing.then) {
        playing.then(function () {
          refreshButton(post);
        }).catch(function () {
          /* Browsers block sound until the visitor interacts. Invite a tap. */
          post.blocked = true;
          refreshButton(post);
        });
      }
    }

    function activate(post) {
      if (!post || post.card === activeCard) return;
      var previous = postFor(activeCard);
      if (previous) pausePost(previous, true);
      activeCard = post.card;
      if (soundOn) playPost(post);
      else refreshButton(post);
    }

    function deactivate(post) {
      if (!post) return;
      pausePost(post, true);
      post.blocked = false;
      refreshButton(post);
      if (activeCard === post.card) activeCard = null;
    }

    if ('IntersectionObserver' in window) {
      var observer = new IntersectionObserver(function (entries) {
        entries.forEach(function (entry) {
          var post = postFor(entry.target);
          if (entry.isIntersecting && entry.intersectionRatio >= 0.55) {
            activate(post);
          } else if (activeCard === entry.target) {
            deactivate(post);
          }
        });
      }, { threshold: [0, 0.55, 0.9], rootMargin: '-8% 0px -8% 0px' });
      cards.forEach(function (card) { observer.observe(card); });
    } else {
      /* Older browsers have no observer; play only what is clicked. */
      posts.forEach(function (post) {
        post.button.addEventListener('click', function () {
          soundOn = true;
          saveSoundPreference(true);
          playPost(post);
        });
      });
      refreshAllButtons();
      return;
    }

    posts.forEach(function (post) {
      if (!post.button) return;

      post.button.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();

        if (!soundOn || post.blocked) {
          soundOn = true;
          post.blocked = false;
          saveSoundPreference(true);
          if (activeCard !== post.card) {
            var previous = postFor(activeCard);
            if (previous) pausePost(previous, true);
            activeCard = post.card;
          }
          playPost(post);
          refreshAllButtons();
          return;
        }

        soundOn = false;
        saveSoundPreference(false);
        posts.forEach(function (item) { pausePost(item, true); });
        refreshAllButtons();
      });

      if (post.video) {
        post.video.addEventListener('pause', function () {
          if (activeCard === post.card) pausePost(post, false);
        });
        post.video.addEventListener('play', function () {
          if (activeCard === post.card && soundOn) playPost(post);
        });
      }
    });

    /* The first tap anywhere lifts the browser's autoplay restriction. */
    function unlockSound() {
      var current = postFor(activeCard);
      if (current && current.blocked) {
        soundOn = true;
        saveSoundPreference(true);
        playPost(current);
      }
    }
    document.addEventListener('pointerdown', unlockSound, { passive: true });
    document.addEventListener('keydown', unlockSound);

    document.addEventListener('visibilitychange', function () {
      if (document.hidden) {
        var current = postFor(activeCard);
        if (current) pausePost(current, false);
        return;
      }
      var visible = postFor(activeCard);
      if (visible && soundOn) playPost(visible);
    });

    refreshAllButtons();
  }

  initComposer();
  initFeedSound();
})();
