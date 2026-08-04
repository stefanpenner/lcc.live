// ========================================
// LCC.live Frontend
// ========================================
// 
// Architecture:
// - No build tools, no frameworks - just modern ES modules
// - ETag-based image updates with true double buffering
// - iOS Safari optimized (avoids content-visibility, async decoding)
// - Scroll-aware (pauses updates during active scrolling)
// - Network-adaptive (adjusts poll rate based on connection speed)
//
// Key Classes:
// - ImageReloader: Handles automatic image updates
// - FullscreenViewer: Manages fullscreen image/iframe viewing
//
// ========================================

// ========================================
// Shared Utilities
// ========================================

function formatTimeAgo(date) {
  const now = Date.now();
  const diff = Math.floor((now - date.getTime()) / 1000);

  if (diff < 60) {
    return '<1m';
  } else if (diff < 3600) {
    return `${Math.floor(diff / 60)}m`;
  } else if (diff < 86400) {
    return `${Math.floor(diff / 3600)}h`;
  } else {
    return `${Math.floor(diff / 86400)}d`;
  }
}

function formatUnixTimeAgo(timestamp) {
  if (!timestamp || timestamp === 0) return 'unknown';
  const now = Math.floor(Date.now() / 1000);
  const diff = now - timestamp;

  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  if (diff < 604800) return `${Math.floor(diff / 86400)}d`;
  if (diff < 31536000) return `${Math.floor(diff / 604800)}w`;
  return `${Math.floor(diff / 31536000)}y`;
}

// ========================================
// ETag-Aware Image Reloading with Double Buffering
// ========================================

class ImageReloader {
  constructor(interval = 3000) {
    this.interval = interval;
    this.observer = null;
    this.abortControllers = new WeakMap();
    this.blobUrls = new WeakMap(); // Track blob URLs for cleanup
    this.lastReloadTime = new WeakMap(); // Track when each image was last reloaded
    this.reloadCooldown = 5000; // Minimum 5 seconds between reloads
    this.isScrolling = false;
    this.scrollTimeout = null;
  }

  async reloadImage(img) {
    try {
      const src = img.dataset.src || img.src;
      img.dataset.src = src;

      if (!img.classList.contains('in-viewport')) {
        return;
      }

      // Don't reload during active scrolling - prevents flicker on iOS
      if (this.isScrolling) {
        return;
      }

      // Check cooldown - prevent rapid reloads on scroll
      const now = Date.now();
      const lastReload = this.lastReloadTime.get(img) || 0;
      if (now - lastReload < this.reloadCooldown) {
        return; // Too soon, skip this reload
      }

      // Cancel any in-flight request for this image
      const oldController = this.abortControllers.get(img);
      if (oldController) oldController.abort();

      const controller = new AbortController();
      this.abortControllers.set(img, controller);
      
      // Update last reload time
      this.lastReloadTime.set(img, now);

      // HEAD request to check ETag without downloading image
      const headResponse = await fetch(src, {
        method: 'HEAD',
        cache: 'no-cache',
        credentials: 'same-origin',
        signal: controller.signal
      });

      if (headResponse.status !== 200) return;

      const etag = headResponse.headers.get('etag');
      const lastModified = headResponse.headers.get('last-modified');

      // Update image age overlay whenever we get a Last-Modified header
      if (lastModified) {
        img.dataset.lastModified = lastModified;
        this.updateImageAge(img);
      }

      // If this is the first check and image is already loaded, just store the ETag
      if (!img.dataset.etag && img.complete && img.naturalWidth > 0) {
        img.dataset.etag = etag;
        return; // Don't swap, image is already displaying correctly
      }

      if (img.dataset.etag === etag) return; // No change

      img.dataset.etag = etag;

      // Fetch the new image
      const response = await fetch(src, {
        cache: 'force-cache',
        credentials: 'same-origin',
        signal: controller.signal
      });

      const blob = await response.blob();
      const newUrl = URL.createObjectURL(blob);

      // True double buffering: preload AND decode before atomic swap
      const tempImg = new Image();
      tempImg.src = newUrl;
      
      try {
        // Wait for image to load and decode
        await tempImg.decode();
        
        // Image is now fully decoded and ready - atomic swap with zero flicker
        requestAnimationFrame(() => {
          const oldSrc = img.src;
          
          // Atomic swap - fully decoded image
          img.src = newUrl;
          this.blobUrls.set(img, newUrl);
          
          // Cleanup old blob URL after swap
          if (oldSrc.startsWith('blob:')) {
            setTimeout(() => URL.revokeObjectURL(oldSrc), 100);
          }
        });
      } catch (decodeError) {
        // Decode failed, clean up
        URL.revokeObjectURL(newUrl);
        console.warn('Failed to decode image:', img.dataset.src, decodeError);
      }
    } catch (error) {
      if (error.name === 'AbortError') return; // Expected cancellation
      console.warn('Failed to reload image:', img.dataset.src, error);
    }
  }

  setupViewportTracking() {
    // Use hysteresis to prevent flickering on iOS Safari
    this.observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          // Add class when entering viewport
          entry.target.classList.add('in-viewport');

          // Eagerly fetch image age for newly visible images
          if (!entry.target.dataset.lastModified) {
            this.fetchImageAge(entry.target);
          }

          // Set last reload time to NOW to prevent immediate reload
          // This prevents flicker when scrolling back to an image
          const now = Date.now();
          const lastReload = this.lastReloadTime.get(entry.target);
          if (!lastReload) {
            // Only set if not already set - prevents updates on re-entry
            this.lastReloadTime.set(entry.target, now);
          }
        } else {
          // Remove class only if fully out of viewport
          // This prevents rapid add/remove during scroll
          if (entry.intersectionRatio === 0) {
            entry.target.classList.remove('in-viewport');
          }
        }
      });
    }, {
      root: null,
      rootMargin: '50px', // Reduced to prevent premature loading on scroll
      threshold: [0, 0.01] // Multiple thresholds for better hysteresis
    });

    document.querySelectorAll('img').forEach(img => {
      this.observer.observe(img);
      // Initialize with current time to prevent initial flicker
      this.lastReloadTime.set(img, Date.now());
    });
  }

  async reloadAll() {
    await Promise.allSettled(
      Array.from(document.querySelectorAll('img')).map(img => this.reloadImage(img))
    );
  }

  getAdaptiveInterval() {
    // Use Network Information API if available
    if ('connection' in navigator) {
      const conn = navigator.connection;
      if (conn.saveData) return 10000; // Slow down if data saver enabled
      if (conn.effectiveType === '4g') return 2000; // Speed up on fast connection
      if (conn.effectiveType === 'slow-2g' || conn.effectiveType === '2g') return 8000;
    }
    return this.interval;
  }

  start() {
    this.setupViewportTracking();
    this.setupScrollTracking();
    
    // Automatic polling with scroll protection
    const reload = async () => {
      await this.reloadAll();
      setTimeout(reload, this.getAdaptiveInterval());
    };
    
    reload();

    // Keep image age badges fresh
    this.imageAgeTimer = setInterval(() => this.updateAllImageAges(), 60000);

    // Reload on visibility change
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') {
        this.reloadAll();
      }
    });

    // Cleanup on unload
    window.addEventListener('beforeunload', () => this.cleanup());
  }

  setupScrollTracking() {
    // Track when user is actively scrolling to prevent flicker
    window.addEventListener('scroll', () => {
      this.isScrolling = true;
      
      // Clear existing timeout
      if (this.scrollTimeout) {
        clearTimeout(this.scrollTimeout);
      }
      
      // Mark as not scrolling after 150ms of no scroll events
      this.scrollTimeout = setTimeout(() => {
        this.isScrolling = false;
      }, 150);
    }, { passive: true });
  }

  async fetchImageAge(img) {
    try {
      const src = img.dataset.src || img.src;
      const response = await fetch(src, {
        method: 'HEAD',
        cache: 'no-cache',
        credentials: 'same-origin',
      });
      if (response.status !== 200) return;
      const lastModified = response.headers.get('last-modified');
      if (lastModified) {
        img.dataset.lastModified = lastModified;
        this.updateImageAge(img);
      }
      // Also seed the etag if not set
      if (!img.dataset.etag) {
        const etag = response.headers.get('etag');
        if (etag) img.dataset.etag = etag;
      }
    } catch {
      // Ignore - this is best-effort
    }
  }

  updateImageAge(img) {
    const lastModified = img.dataset.lastModified;
    if (!lastModified) return;

    const feed = img.closest('camera-feed');
    if (!feed) return;

    let badge = feed.querySelector('.image-age');
    if (!badge) {
      badge = document.createElement('span');
      badge.className = 'image-age';
      feed.appendChild(badge);
    }

    const date = new Date(lastModified);
    if (isNaN(date.getTime())) return;

    badge.textContent = formatTimeAgo(date);
  }

  updateAllImageAges() {
    document.querySelectorAll('img[data-last-modified]').forEach(img => {
      this.updateImageAge(img);
    });
  }

  cleanup() {
    this.observer?.disconnect();
    if (this.imageAgeTimer) clearInterval(this.imageAgeTimer);

    // Revoke all blob URLs
    document.querySelectorAll('img').forEach(img => {
      const blobUrl = this.blobUrls.get(img);
      if (blobUrl) {
        URL.revokeObjectURL(blobUrl);
      }
    });
  }
}

// ========================================
// Fullscreen Image Viewer
// ========================================

class FullscreenViewer {
  constructor() {
    this.overlay = null;
    this.items = []; // Both images and iframes
    this.currentIndex = -1;
    this.touchStartX = 0;
    this.touchStartY = 0;
    this.scrollPosition = 0;
    this.setupOverlay();
    this.setupEventListeners();
  }

  setupOverlay() {
    // Use existing the-overlay element from template
    this.overlay = document.querySelector('the-overlay');
    if (!this.overlay) {
      // Fallback: create if not found
      this.overlay = document.createElement('the-overlay');
      this.overlay.setAttribute('role', 'dialog');
      this.overlay.setAttribute('aria-modal', 'true');
      this.overlay.setAttribute('aria-label', 'Enlarged camera view');
      document.body.appendChild(this.overlay);
    }

    // Close on click
    this.overlay.addEventListener('click', (e) => {
      // Don't close if clicking on video controls or iframe
      if (e.target.tagName === 'VIDEO') {
        // Let video controls handle the click - don't close overlay
        return;
      }
      if (e.target.tagName === 'IFRAME') {
        // Let iframe handle the click - don't close overlay
        return;
      }
      // Close on background or image click
      if (e.target === this.overlay || e.target.tagName === 'IMG') {
        this.close();
      }
    });

    // Touch gestures for mobile
    this.setupTouchGestures();
  }

  setupTouchGestures() {
    this.overlay.addEventListener('touchstart', (e) => {
      this.touchStartX = e.changedTouches[0].screenX;
      this.touchStartY = e.changedTouches[0].screenY;
    }, { passive: true });

    this.overlay.addEventListener('touchend', (e) => {
      const touchEndX = e.changedTouches[0].screenX;
      const touchEndY = e.changedTouches[0].screenY;
      
      const deltaX = this.touchStartX - touchEndX;
      const deltaY = this.touchStartY - touchEndY;
      const minSwipeDistance = 50;

      // Horizontal swipe
      if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > minSwipeDistance) {
        if (deltaX > 0) {
          this.next(); // Swipe left
        } else {
          this.previous(); // Swipe right
        }
      }
      // Vertical swipe down to close
      else if (deltaY < 0 && Math.abs(deltaY) > minSwipeDistance) {
        this.close();
      }
    }, { passive: true });
  }

  setupEventListeners() {
    // Click on images or iframes to open fullscreen
    document.body.addEventListener('click', (e) => {
      // Handle clicks on links containing images (camera-feed images)
      const link = e.target.closest('a');
      if (link && !link.closest('the-overlay')) {
        const img = link.querySelector('img');
        if (img) {
          e.preventDefault();
          e.stopPropagation();
          this.open(img);
          return;
        }
      }
      
      // Handle direct clicks on images, iframes, or videos
      const img = e.target.closest('img');
      const iframe = e.target.closest('iframe');
      const video = e.target.closest('video');
      
      if (img && !img.closest('the-overlay') && !img.closest('a')) {
        e.preventDefault();
        this.open(img);
      } else if (iframe && !iframe.closest('the-overlay')) {
        e.preventDefault();
        this.open(iframe);
      } else if (video && !video.closest('the-overlay')) {
        e.preventDefault();
        this.open(video);
      }
    });

    // Keyboard navigation
    document.addEventListener('keydown', (e) => {
      if (!this.isOpen()) return;

      switch (e.key) {
        case 'Escape':
          this.close();
          e.preventDefault();
          break;
        case 'ArrowLeft':
        case 'Left':
          this.previous();
          e.preventDefault();
          break;
        case 'ArrowRight':
        case 'Right':
          this.next();
          e.preventDefault();
          break;
      }
    });
  }

  isOpen() {
    return this.overlay.style.display === 'flex' || this.overlay.style.display === 'block';
  }

  open(element) {
    // Get all images, iframes, and videos in the page
    const images = Array.from(document.querySelectorAll('img')).filter(
      i => !i.closest('the-overlay')
    );
    const iframes = Array.from(document.querySelectorAll('iframe')).filter(
      i => !i.closest('the-overlay')
    );
    const videos = Array.from(document.querySelectorAll('video')).filter(
      v => !v.closest('the-overlay')
    );
    
    // Combine and sort by DOM order
    this.items = [...images, ...iframes, ...videos].sort((a, b) => {
      return a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1;
    });
    
    this.currentIndex = this.items.indexOf(element);

    if (this.currentIndex === -1) return;

    this.showItem();
  }

  showItem() {
    if (this.currentIndex < 0 || this.currentIndex >= this.items.length) return;

    const sourceElement = this.items[this.currentIndex];
    
    // Clear overlay
    this.overlay.innerHTML = '';
    
    // Clone and display element (image, iframe, or video)
    if (sourceElement.tagName === 'IMG') {
      const img = document.createElement('img');
      img.src = sourceElement.src;
      img.alt = sourceElement.alt;
      this.overlay.appendChild(img);
    } else if (sourceElement.tagName === 'IFRAME') {
      const iframe = document.createElement('iframe');
      iframe.src = sourceElement.src;
      iframe.title = sourceElement.title || sourceElement.getAttribute('aria-label');
      iframe.allow = 'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share; fullscreen';
      iframe.allowFullscreen = true;
      iframe.style.cssText = `
        width: 90vw;
        height: 90vh;
        max-width: 100%;
        max-height: 100%;
        border: none;
        border-radius: var(--radius-sm);
      `;
      this.overlay.appendChild(iframe);
    } else if (sourceElement.tagName === 'VIDEO') {
      const video = document.createElement('video');
      video.src = sourceElement.src;
      video.controls = true;
      video.autoplay = true;
      video.playsInline = true;
      video.style.cssText = `
        width: 90vw;
        height: 90vh;
        max-width: 100%;
        max-height: 100%;
        border-radius: var(--radius-sm);
      `;
      
      // Prevent clicks on video from closing overlay (let controls work)
      video.addEventListener('click', (e) => {
        e.stopPropagation();
      });
      
      // Enable native browser fullscreen on double-click
      video.addEventListener('dblclick', async (e) => {
        e.stopPropagation(); // Prevent overlay close
        e.preventDefault();
        try {
          if (video.requestFullscreen) {
            await video.requestFullscreen();
          } else if (video.webkitRequestFullscreen) {
            await video.webkitRequestFullscreen();
          } else if (video.webkitEnterFullscreen) {
            // iOS Safari
            video.webkitEnterFullscreen();
          } else if (video.mozRequestFullScreen) {
            await video.mozRequestFullScreen();
          } else if (video.msRequestFullscreen) {
            await video.msRequestFullscreen();
          }
        } catch (error) {
          console.warn('Fullscreen request failed:', error);
        }
      });
      
      // Also handle fullscreen button click in video controls
      video.addEventListener('webkitbeginfullscreen', () => {
        // iOS Safari fullscreen started
      });
      
      video.addEventListener('webkitendfullscreen', () => {
        // iOS Safari fullscreen ended
      });
      
      this.overlay.appendChild(video);
    }
    
    // Add caption (Name + Temp) from source
    const cameraFeed = sourceElement.closest('camera-feed');
    if (cameraFeed) {
      const footer = document.createElement('div');
      footer.className = 'overlay-footer';
      
      const name = cameraFeed.querySelector('h4')?.textContent;
      if (name) {
        const nameEl = document.createElement('h4');
        nameEl.textContent = name;
        footer.appendChild(nameEl);
      }

      const temp = cameraFeed.querySelector('.camera-temp');
      if (temp) {
        footer.appendChild(temp.cloneNode(true));
      }

      for (const chip of cameraFeed.querySelectorAll('.camera-weather-chip')) {
        footer.appendChild(chip.cloneNode(true));
      }
      
      if (footer.children.length > 0) {
        this.overlay.appendChild(footer);
      }
    }

    this.overlay.style.display = 'flex';
    
    // Store scroll position and prevent body scroll (iOS-specific handling)
    this.scrollPosition = window.scrollY || window.pageYOffset;
    document.body.style.overflow = 'hidden';
    document.body.style.position = 'fixed';
    document.body.style.width = '100%';
    document.body.style.height = '100%';
    document.body.style.top = `-${this.scrollPosition}px`;
    
    // Prefetch adjacent items for smoother navigation
    this.prefetchAdjacent();
  }

  prefetchAdjacent() {
    // Prefetch next and previous items (images only, iframes load on demand)
    const indicesToPrefetch = [this.currentIndex - 1, this.currentIndex + 1]
      .filter(i => i >= 0 && i < this.items.length);
    
    indicesToPrefetch.forEach(i => {
      const element = this.items[i];
      // Only prefetch images, not iframes
      if (element.tagName === 'IMG') {
        const link = document.createElement('link');
        link.rel = 'prefetch';
        link.as = 'image';
        link.href = element.src;
        document.head.appendChild(link);
        
        // Clean up after a short delay
        setTimeout(() => link.remove(), 5000);
      }
    });
  }

  close() {
    this.overlay.style.display = 'none';
    this.overlay.innerHTML = '';
    
    // Restore scroll position (iOS-specific handling)
    const scrollPos = this.scrollPosition || 0;
    document.body.style.removeProperty('overflow');
    document.body.style.removeProperty('position');
    document.body.style.removeProperty('width');
    document.body.style.removeProperty('height');
    document.body.style.removeProperty('top');
    
    // Restore scroll position after clearing position:fixed
    requestAnimationFrame(() => {
      window.scrollTo(0, scrollPos);
    });
    
    this.currentIndex = -1;
  }

  next() {
    if (this.currentIndex < this.items.length - 1) {
      this.currentIndex++;
      this.showItem();
    }
  }

  previous() {
    if (this.currentIndex > 0) {
      this.currentIndex--;
      this.showItem();
    }
  }
}

// ========================================
// Share Functionality
// ========================================

class ShareHandler {
  constructor() {
    this.setupShareButtons();
  }

  setupShareButtons() {
    document.body.addEventListener('click', async (e) => {
      const shareButton = e.target.closest('.share-button');
      if (!shareButton) return;

      e.preventDefault();
      e.stopPropagation();

      const cameraId = shareButton.dataset.cameraId;
      const cameraName = shareButton.dataset.cameraName || 'Camera';
      const cameraKind = shareButton.dataset.cameraKind || '';
      const isVideo = cameraKind === 'iframe';
      
      // For videos/iframes, share the camera page URL
      if (isVideo) {
        await this.shareCameraPage(cameraId, cameraName, shareButton);
        return;
      }
      
      // For images, share the image file itself
      await this.shareImage(cameraId, cameraName, shareButton);
    });
  }

  async shareCameraPage(cameraId, cameraName, shareButton) {
    // Get the current page URL or camera-specific URL
    let url = window.location.href;
    let title = document.title;
    
    // If we have a camera ID, construct the camera URL
    if (cameraId) {
      // Check if we're on the canyon page or camera detail page
      const currentPath = window.location.pathname;
      if (currentPath.startsWith('/camera/')) {
        // Already on camera page, use current URL
        url = window.location.href;
      } else {
        // On canyon page, link to camera detail page using slug
        // Generate slug from camera name
        const slug = cameraName.toLowerCase()
          .replace(/[\s_]+/g, '-')
          .replace(/[^a-z0-9-]/g, '')
          .replace(/-+/g, '-')
          .replace(/^-|-$/g, '');
        url = `${window.location.origin}/camera/${slug}`;
        title = `${cameraName} | ${document.title.split('|')[1] || 'Live Camera'}`;
      }
    }

    // Try Web Share API first
    if (navigator.share) {
      try {
        await navigator.share({
          url: url,
          title: title,
        });
        return; // Successfully shared
      } catch (error) {
        // User cancelled or share failed, fall back to clipboard
        if (error.name !== 'AbortError') {
          console.warn('Web Share API failed:', error);
        } else {
          return; // User cancelled, don't fall back
        }
      }
    }

    // Fallback: Copy URL to clipboard
    await this.fallbackCopyUrl(url, shareButton, 'Share this camera');
  }

  async shareImage(cameraId, cameraName, shareButton) {
    if (!cameraId) {
      console.error('Cannot share image: camera ID missing');
      return;
    }

    const imageUrl = `/image/${cameraId}`;
    
    try {
      // Fetch the image as a blob
      const response = await fetch(imageUrl);
      if (!response.ok) {
        throw new Error(`Failed to fetch image: ${response.status}`);
      }
      
      const blob = await response.blob();
      
      // Determine file extension from content type or default to jpg
      let extension = 'jpg';
      const contentType = response.headers.get('content-type');
      if (contentType) {
        if (contentType.includes('png')) extension = 'png';
        else if (contentType.includes('gif')) extension = 'gif';
        else if (contentType.includes('webp')) extension = 'webp';
      }
      
      // Create a File object with a meaningful name
      const fileName = `${cameraName.replace(/[^a-z0-9]/gi, '_').toLowerCase()}.${extension}`;
      const file = new File([blob], fileName, { type: blob.type });
      
      // Try Web Share API with file
      if (navigator.share && navigator.canShare && navigator.canShare({ files: [file] })) {
        try {
          await navigator.share({
            files: [file],
            title: cameraName,
          });
          return; // Successfully shared
        } catch (error) {
          // User cancelled or share failed, fall back to URL sharing
          if (error.name !== 'AbortError') {
            console.warn('Web Share API with file failed:', error);
          } else {
            return; // User cancelled, don't fall back
          }
        }
      }
      
      // Fallback: Share image URL instead
      const imageFullUrl = `${window.location.origin}${imageUrl}`;
      if (navigator.share) {
        try {
          await navigator.share({
            url: imageFullUrl,
            title: cameraName,
          });
          return;
        } catch (error) {
          if (error.name !== 'AbortError') {
            console.warn('Web Share API with URL failed:', error);
          } else {
            return;
          }
        }
      }
      
      // Final fallback: Copy image URL to clipboard
      await this.fallbackCopyUrl(imageFullUrl, shareButton, 'Share this image');
    } catch (error) {
      console.error('Failed to share image:', error);
      // Last resort: show error message
      const originalTitle = shareButton.getAttribute('title');
      shareButton.setAttribute('title', 'Failed to share');
      setTimeout(() => {
        shareButton.setAttribute('title', originalTitle || 'Share this image');
      }, 2000);
    }
  }

  async fallbackCopyUrl(url, shareButton, defaultTitle) {
    try {
      // Copy just the URL (not text) to clipboard for easy pasting
      await navigator.clipboard.writeText(url);
      // Show visual feedback
      const originalTitle = shareButton.getAttribute('title');
      shareButton.setAttribute('title', 'Copied!');
      shareButton.style.opacity = '1';
      setTimeout(() => {
        shareButton.setAttribute('title', originalTitle || defaultTitle);
        shareButton.style.opacity = '';
      }, 2000);
    } catch (error) {
      console.error('Failed to copy URL:', error);
      // Last resort: show URL in alert
      alert(`Share this:\n${url}`);
    }
  }
}

// ========================================
// Fullscreen Button Handler
// ========================================

class FullscreenButtonHandler {
  constructor(viewer) {
    this.viewer = viewer;
    this.setupFullscreenButtons();
  }

  setupFullscreenButtons() {
    document.body.addEventListener('click', (e) => {
      const fullscreenButton = e.target.closest('.fullscreen-button');
      if (!fullscreenButton) return;

      e.preventDefault();
      e.stopPropagation();

      const cameraId = fullscreenButton.dataset.cameraId;
      
      // Find the camera feed element
      const cameraFeed = fullscreenButton.closest('camera-feed');
      if (!cameraFeed) return;

      // Find the image or iframe inside the camera feed
      const img = cameraFeed.querySelector('img');
      const iframe = cameraFeed.querySelector('iframe');
      const video = cameraFeed.querySelector('video');
      
      const element = img || iframe || video;
      if (element) {
        // Open in overlay viewer
        this.viewer.open(element);
      }
    });
  }
}

// ========================================
// UDOT Data Poller
// ========================================

class UDOTPoller {
  constructor(canyonName, interval = 60000) {
    this.canyonName = canyonName;
    this.interval = interval;
    this.pollTimer = null;
    this.timeAgoTimer = null; // Timer for continuous time-ago updates
    this.retryDelay = 60000; // Start with 60s retry delay
    this.maxRetryDelay = 300000; // Max 5 minutes
  }

  start() {
    // Start polling
    this.poll();
    // Start continuous time-ago updates every minute
    this.startTimeAgoUpdates();
  }

  async poll() {
    try {
      const response = await fetch(`/api/canyon/${this.canyonName}/udot`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json();
      
      // Reset retry delay on success
      this.retryDelay = 60000;

      // Update road conditions (always call to handle empty arrays)
      if (data.roadConditions && Array.isArray(data.roadConditions)) {
        this.updateRoadConditions(data.roadConditions);
      }

      // Update per-camera weather chips
      if (data.weatherStations) {
        this.updateWeatherChips(data.weatherStations);
      }

      // UDOT events, UAC danger, Alta parking
      if (Array.isArray(data.events)) {
        this.updateEvents(data.events);
      }
      this.updateAvalancheDanger(data.avalancheDanger || null);
      this.updateAltaStatus(data.altaStatus || null);

      // Schedule next poll
      this.pollTimer = setTimeout(() => this.poll(), this.interval);
    } catch (error) {
      console.warn('UDOT poll failed:', error);
      // Exponential backoff on errors
      const delay = Math.min(this.retryDelay, this.maxRetryDelay);
      this.retryDelay = Math.min(this.retryDelay * 1.5, this.maxRetryDelay);
      this.pollTimer = setTimeout(() => this.poll(), delay);
    }
  }

  hasActiveRestriction(restriction) {
    if (!restriction) return false;
    const r = String(restriction).trim().toLowerCase();
    return r !== '' && r !== 'none' && r !== 'no restrictions' && r !== 'n/a';
  }

  restrictionLabel(restriction) {
    return restriction && String(restriction).trim() !== '' ? String(restriction).trim() : 'none';
  }

  roadCardInnerHTML(cond) {
    const road = this.escapeHtml(cond.RoadCondition);
    const weather = this.escapeHtml(cond.WeatherCondition);
    const name = this.escapeHtml(cond.RoadwayName);
    const ago = this.escapeHtml(this.formatTimeAgo(cond.LastUpdated));
    const restriction = this.restrictionLabel(cond.Restriction);
    const restrictionEsc = this.escapeHtml(restriction);
    const restrictionClass = this.hasActiveRestriction(restriction)
      ? 'road-chip road-chip-warn'
      : 'road-chip road-chip-muted';

    return `
      <h3 class="road-condition-title" title="${name}">${name}</h3>
      <div class="road-condition-content">
        <span class="road-chip" data-kind="road" data-condition="${road}" aria-label="Road: ${road}">${road}</span>
        <span class="road-chip" data-kind="weather" data-condition="${weather}" aria-label="Weather: ${weather}">${weather}</span>
        <span class="${restrictionClass}" data-kind="restriction" aria-label="Restriction: ${restrictionEsc}">${restrictionEsc}</span>
        <time class="road-chip road-chip-muted road-condition-updated" datetime="${cond.LastUpdated}" data-last-updated="${cond.LastUpdated}" aria-label="Updated ${ago} ago">
          <span class="road-condition-time-ago">${ago}</span>
        </time>
      </div>
    `;
  }

  updateRoadChip(card, kind, value, labelPrefix) {
    const chip = card.querySelector(`.road-chip[data-kind="${kind}"]`);
    if (!chip || value == null) return;
    if (chip.textContent !== value) {
      chip.textContent = value;
      chip.setAttribute('data-condition', value);
      chip.setAttribute('aria-label', `${labelPrefix}: ${value}`);
    }
  }

  updateRoadConditions(conditions) {
    const banner = document.querySelector('.road-conditions-banner');
    if (!banner) return;

    // Server already filters unwanted road conditions
    if (!conditions || conditions.length === 0) {
      banner.hidden = true;
      banner.style.display = '';
      return;
    }

    banner.hidden = false;
    banner.style.display = '';

    const existingCards = new Map();
    banner.querySelectorAll('.road-conditions-card').forEach(card => {
      const id = card.getAttribute('data-condition-id');
      if (id) {
        existingCards.set(parseInt(id, 10), card);
      }
    });

    const newConditionsMap = new Map();
    conditions.forEach(cond => {
      newConditionsMap.set(cond.Id, cond);
    });

    existingCards.forEach((card, id) => {
      if (!newConditionsMap.has(id)) {
        card.remove();
      }
    });

    conditions.forEach(cond => {
      let card = existingCards.get(cond.Id);

      if (!card) {
        card = document.createElement('div');
        card.className = 'road-conditions-card';
        card.setAttribute('data-condition-id', cond.Id);
        card.innerHTML = this.roadCardInnerHTML(cond);
        banner.appendChild(card);
        return;
      }

      const title = card.querySelector('.road-condition-title');
      if (title && title.textContent !== cond.RoadwayName) {
        title.textContent = cond.RoadwayName;
        title.setAttribute('title', cond.RoadwayName);
      }

      this.updateRoadChip(card, 'road', cond.RoadCondition, 'Road');
      this.updateRoadChip(card, 'weather', cond.WeatherCondition, 'Weather');

      // Restriction chip always present (muted when none)
      const content = card.querySelector('.road-condition-content');
      let restrictionChip = card.querySelector('.road-chip[data-kind="restriction"]');
      const restriction = this.restrictionLabel(cond.Restriction);
      if (!restrictionChip && content) {
        restrictionChip = document.createElement('span');
        restrictionChip.setAttribute('data-kind', 'restriction');
        const updatedTimeEl = content.querySelector('.road-condition-updated');
        if (updatedTimeEl) {
          content.insertBefore(restrictionChip, updatedTimeEl);
        } else {
          content.appendChild(restrictionChip);
        }
      }
      if (restrictionChip) {
        restrictionChip.textContent = restriction;
        restrictionChip.setAttribute('aria-label', `Restriction: ${restriction}`);
        restrictionChip.className = this.hasActiveRestriction(restriction)
          ? 'road-chip road-chip-warn'
          : 'road-chip road-chip-muted';
      }

      let updatedTime = card.querySelector('.road-condition-updated');
      if (!updatedTime && content) {
        updatedTime = document.createElement('time');
        updatedTime.className = 'road-chip road-chip-muted road-condition-updated';
        content.appendChild(updatedTime);
      }
      if (!updatedTime) return;

      const ago = this.formatTimeAgo(cond.LastUpdated);
      updatedTime.setAttribute('datetime', cond.LastUpdated);
      updatedTime.setAttribute('data-last-updated', cond.LastUpdated);
      updatedTime.setAttribute('aria-label', `Updated ${ago} ago`);

      let timeAgoSpan = updatedTime.querySelector('.road-condition-time-ago');
      if (!timeAgoSpan) {
        updatedTime.textContent = '';
        timeAgoSpan = document.createElement('span');
        timeAgoSpan.className = 'road-condition-time-ago';
        updatedTime.appendChild(timeAgoSpan);
      }
      if (timeAgoSpan.textContent !== ago) {
        timeAgoSpan.textContent = ago;
      }
    });
  }

  ensureTopBarChips() {
    let chips = document.querySelector('.top-bar-chips');
    if (chips) return chips;
    const topBar = document.querySelector('.top-bar');
    if (!topBar) return null;
    chips = document.createElement('div');
    chips.className = 'top-bar-chips';
    chips.setAttribute('role', 'group');
    chips.setAttribute('aria-label', 'Status chips');
    const banner = topBar.querySelector('.road-conditions-banner');
    if (banner) {
      topBar.insertBefore(chips, banner);
    } else {
      topBar.appendChild(chips);
    }
    return chips;
  }

  eventLabel(ev) {
    const raw = (ev.Name && String(ev.Name).trim()) || (ev.Description && String(ev.Description).trim()) || 'Event';
    if (raw.length <= 40) return raw;
    return raw.slice(0, 39) + '…';
  }

  eventIsWarn(ev) {
    if (ev.IsFullClosure) return true;
    const s = String(ev.Severity || '').trim().toLowerCase();
    return s === 'high' || s === 'severe' || s === 'critical' || s === 'major';
  }

  updateEvents(events) {
    const host = this.ensureTopBarChips();
    if (!host) return;

    let strip = document.getElementById('events-strip');
    if (!strip) {
      strip = document.createElement('div');
      strip.id = 'events-strip';
      strip.className = 'events-strip';
      strip.setAttribute('role', 'list');
      strip.setAttribute('aria-label', 'Traffic events');
      host.appendChild(strip);
    }

    if (!events || events.length === 0) {
      strip.innerHTML = '';
      strip.hidden = true;
      strip.setAttribute('data-event-count', '0');
      return;
    }

    strip.hidden = false;
    strip.setAttribute('data-event-count', String(events.length));

    const maxShow = 3;
    const shown = events.slice(0, maxShow);
    const parts = shown.map((ev) => {
      const label = this.escapeHtml(this.eventLabel(ev));
      const full = this.escapeHtml((ev.Name && String(ev.Name).trim()) || (ev.Description && String(ev.Description).trim()) || 'Event');
      const warn = this.eventIsWarn(ev) ? ' road-chip-warn' : '';
      const aria = full + (ev.IsFullClosure ? ' (full closure)' : '');
      return `<span class="status-chip event-chip${warn}" role="listitem" data-event-id="${this.escapeHtml(String(ev.ID || ''))}" title="${full}" aria-label="${aria}">${label}</span>`;
    });

    if (events.length > maxShow) {
      const more = events.length - maxShow;
      parts.push(`<span class="status-chip event-chip event-chip-more" id="events-more" role="listitem" aria-label="${more} more events">+${more}</span>`);
    }

    strip.innerHTML = parts.join('');
  }

  uacDangerClass(level, danger) {
    const d = String(danger || '').trim().toLowerCase();
    if ((level == null || level <= 0) && (!d || d === 'no rating' || d === 'none' || d === 'n/a')) {
      return 'uac-none';
    }
    switch (Number(level)) {
      case 1: return 'uac-low';
      case 2: return 'uac-moderate';
      case 3: return 'uac-considerable';
      case 4:
      case 5: return 'uac-high';
      default:
        if (d === 'low') return 'uac-low';
        if (d === 'moderate') return 'uac-moderate';
        if (d === 'considerable') return 'uac-considerable';
        if (d === 'high' || d === 'extreme') return 'uac-high';
        return 'uac-none';
    }
  }

  uacDangerLabel(danger) {
    const d = String(danger || '').trim();
    if (!d || d.toLowerCase() === 'no rating') return 'No rating';
    return d.replace(/\b\w/g, (c) => c.toUpperCase());
  }

  updateAvalancheDanger(ad) {
    const host = this.ensureTopBarChips();
    if (!host) return;

    let chip = document.getElementById('uac-chip');
    if (!chip) {
      chip = document.createElement('a');
      chip.id = 'uac-chip';
      chip.className = 'status-chip uac-chip uac-none';
      chip.target = '_blank';
      chip.rel = 'noopener noreferrer';
      chip.href = 'https://utahavalanchecenter.org/forecast/salt-lake';
      host.insertBefore(chip, host.firstChild);
    }

    if (!ad) {
      chip.hidden = true;
      return;
    }

    const label = this.uacDangerLabel(ad.danger);
    const level = ad.dangerLevel;
    const link = ad.link || 'https://utahavalanchecenter.org/forecast/salt-lake';
    const cls = this.uacDangerClass(level, ad.danger);
    chip.hidden = false;
    chip.href = link;
    chip.className = `status-chip uac-chip ${cls}`;
    chip.setAttribute('data-danger-level', String(level ?? ''));
    const advice = ad.travelAdvice ? `. ${ad.travelAdvice}` : '';
    chip.setAttribute('aria-label', `Utah Avalanche Center Salt Lake: ${label}${advice}`);
    chip.textContent = `UAC: ${label}`;
  }

  altaParkingWarn(status) {
    const s = String(status || '').trim().toLowerCase();
    return s === 'full' || s === 'closed' || s === 'limited' || s.includes('full') || s.includes('closed');
  }

  updateAltaStatus(st) {
    // Alta chips only exist on LCC; if SSR omitted them, skip
    if (this.canyonName !== 'LCC') return;

    const host = this.ensureTopBarChips();
    if (!host) return;

    let parking = document.getElementById('alta-parking-chip');
    let road = document.getElementById('alta-road-chip');

    if (!parking) {
      parking = document.createElement('span');
      parking.id = 'alta-parking-chip';
      parking.className = 'status-chip alta-chip';
      parking.hidden = true;
      // Insert after UAC chip when present
      const uac = document.getElementById('uac-chip');
      if (uac && uac.nextSibling) {
        host.insertBefore(parking, uac.nextSibling);
      } else {
        host.appendChild(parking);
      }
    }
    if (!road) {
      road = document.createElement('span');
      road.id = 'alta-road-chip';
      road.className = 'status-chip alta-chip alta-road-chip';
      road.hidden = true;
      if (parking.nextSibling) {
        host.insertBefore(road, parking.nextSibling);
      } else {
        host.appendChild(road);
      }
    }

    if (!st) {
      parking.hidden = true;
      road.hidden = true;
      return;
    }

    if (st.parkingStatus) {
      parking.hidden = false;
      parking.textContent = `Alta park: ${st.parkingStatus}`;
      parking.setAttribute('data-status', st.parkingStatus);
      const msg = st.parkingMessage ? `. ${st.parkingMessage}` : '';
      parking.setAttribute('aria-label', `Alta parking: ${st.parkingStatus}${msg}`);
      parking.className = this.altaParkingWarn(st.parkingStatus)
        ? 'status-chip alta-chip road-chip-warn'
        : 'status-chip alta-chip';
    } else {
      parking.hidden = true;
    }

    if (st.roadStatus) {
      road.hidden = false;
      road.textContent = `Alta road: ${st.roadStatus}`;
      road.setAttribute('data-status', st.roadStatus);
      const msg = st.roadMessage ? `. ${st.roadMessage}` : '';
      road.setAttribute('aria-label', `Alta road: ${st.roadStatus}${msg}`);
      road.className = this.altaParkingWarn(st.roadStatus)
        ? 'status-chip alta-chip alta-road-chip road-chip-warn'
        : 'status-chip alta-chip alta-road-chip';
    } else {
      road.hidden = true;
    }
  }

  updateWeatherChips(stations) {
    for (const [cameraId, ws] of Object.entries(stations)) {
      const feed = document.querySelector(`camera-feed[data-camera-id="${cameraId}"]`);
      if (!feed) continue;

      const footer = feed.querySelector('.camera-footer');
      if (!footer) continue;

      const actions = footer.querySelector('.camera-actions');

      // Remove existing weather chips
      for (const el of footer.querySelectorAll('.camera-temp, .camera-weather-chip')) {
        el.remove();
      }

      // Hide temps older than 2h (matches server isStale) — kills dead multi-month feeds
      if (this.isWeatherStale(ws.LastUpdated)) {
        continue;
      }

      const chips = [];

      if (ws.AirTemperature) {
        chips.push(`<span class="camera-temp">${this.roundTemp(ws.AirTemperature)}°F</span>`);
      }
      if (ws.WindSpeedAvg) {
        const dir = ws.WindDirection ? ` ${this.escapeHtml(ws.WindDirection)}` : '';
        chips.push(`<span class="camera-weather-chip">${this.roundTemp(ws.WindSpeedAvg)} mph${dir}</span>`);
      }
      if (ws.SurfaceStatus) {
        chips.push(`<span class="camera-weather-chip">${this.escapeHtml(ws.SurfaceStatus)}</span>`);
      }
      if (ws.Precipitation) {
        chips.push(`<span class="camera-weather-chip">${this.precipIcon(ws.AirTemperature)}</span>`);
      }

      // Insert chips before camera-actions
      if (chips.length > 0) {
        const frag = document.createRange().createContextualFragment(chips.join(''));
        if (actions) {
          footer.insertBefore(frag, actions);
        } else {
          footer.appendChild(frag);
        }
      }
    }
  }

  roundTemp(value) {
    const n = parseFloat(value);
    return isNaN(n) ? value : Math.round(n).toString();
  }

  isWeatherStale(timestamp) {
    if (!timestamp) return true;
    const age = Date.now() / 1000 - timestamp;
    return age > 7200; // 2h — matches server isStale (UDOT RWIS cadence)
  }

  precipIcon(airTemp) {
    const svgSnow = '<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="12" y1="2" x2="12" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="5" y1="5" x2="19" y2="19"/><line x1="19" y1="5" x2="5" y2="19"/></svg>';
    const svgRain = '<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2.69l5.66 5.66a8 8 0 1 1-11.31 0z"/></svg>';
    const svgMixed = '<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="6" y1="2" x2="6" y2="12"/><line x1="1" y1="7" x2="11" y2="7"/><line x1="3" y1="4" x2="9" y2="10"/><line x1="9" y1="4" x2="3" y2="10"/><path d="M17 11l4 4a5.5 5.5 0 1 1-8 0z" fill="currentColor" stroke="none"/></svg>';

    if (!airTemp) return svgRain;
    const temp = parseFloat(airTemp);
    if (isNaN(temp)) return svgRain;
    if (temp < 35) return svgSnow;
    if (temp <= 40) return svgMixed;
    return svgRain;
  }

  escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  formatUnixTime(timestamp) {
    if (!timestamp || timestamp === 0) {
      return 'Unknown';
    }
    const date = new Date(timestamp * 1000);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true
    });
  }

  formatTimeAgo(timestamp) {
    return formatUnixTimeAgo(timestamp);
  }

  startTimeAgoUpdates() {
    // Update all time-ago displays every minute
    const updateAllTimeAgo = () => {
      const banner = document.querySelector('.road-conditions-banner');
      if (!banner) return;

      banner.querySelectorAll('.road-condition-updated').forEach(updatedTime => {
        const timestamp = parseInt(updatedTime.getAttribute('data-last-updated') || '0');
        if (timestamp > 0) {
          const ago = this.formatTimeAgo(timestamp);
          const timeAgoSpan = updatedTime.querySelector('.road-condition-time-ago');
          if (timeAgoSpan) {
            timeAgoSpan.textContent = ago;
          }
          updatedTime.setAttribute('aria-label', `Updated ${ago} ago`);
        }
      });
    };

    // Update immediately, then every minute
    updateAllTimeAgo();
    this.timeAgoTimer = setInterval(updateAllTimeAgo, 60000);
  }

  stop() {
    if (this.pollTimer) {
      clearTimeout(this.pollTimer);
      this.pollTimer = null;
    }
    if (this.timeAgoTimer) {
      clearInterval(this.timeAgoTimer);
      this.timeAgoTimer = null;
    }
  }
}

// ========================================
// Initialize
// ========================================

document.addEventListener('DOMContentLoaded', () => {
  const reloader = new ImageReloader(3000);
  reloader.start();

  const viewer = new FullscreenViewer();
  const shareHandler = new ShareHandler();
  const fullscreenHandler = new FullscreenButtonHandler(viewer);

  // Start UDOT polling if on canyon page
  const canyonNav = document.querySelector('.canyon-nav');
  if (canyonNav) {
    // Determine canyon name from active tab or URL
    const activeTab = canyonNav.querySelector('.active');
    let canyonName = 'LCC'; // default
    if (activeTab) {
      canyonName = activeTab.textContent.trim();
    } else if (window.location.pathname.includes('/bcc')) {
      canyonName = 'BCC';
    }

    const poller = new UDOTPoller(canyonName);
    poller.start();
  }
});
