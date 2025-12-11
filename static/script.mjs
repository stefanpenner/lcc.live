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

  cleanup() {
    this.observer?.disconnect();
    
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

  updateRoadConditions(conditions) {
    const banner = document.querySelector('.road-conditions-banner');
    if (!banner) return;

    // Server already filters unwanted road conditions
    if (!conditions || conditions.length === 0) {
      // Hide banner if no conditions
      banner.style.display = 'none';
      return;
    }

    banner.style.display = '';

    // Create a map of existing cards by condition ID
    const existingCards = new Map();
    banner.querySelectorAll('.road-conditions-card').forEach(card => {
      const id = card.getAttribute('data-condition-id');
      if (id) {
        existingCards.set(parseInt(id), card);
      }
    });

    // Create a map of new conditions by ID
    const newConditionsMap = new Map();
    conditions.forEach(cond => {
      newConditionsMap.set(cond.Id, cond);
    });

    // Remove cards that no longer exist
    existingCards.forEach((card, id) => {
      if (!newConditionsMap.has(id)) {
        card.remove();
      }
    });

    // Update or create cards for each condition
    conditions.forEach(cond => {
      let card = existingCards.get(cond.Id);
      
      if (!card) {
        // Create new card
        card = document.createElement('div');
        card.className = 'road-conditions-card';
        card.setAttribute('data-condition-id', cond.Id);
        card.innerHTML = `
          <h3 class="road-condition-title">${this.escapeHtml(cond.RoadwayName)}</h3>
          <div class="road-condition-badge">
            <span class="road-condition-badge-label">Road</span>
            <span class="road-condition-badge-value" data-condition="${this.escapeHtml(cond.RoadCondition)}">${this.escapeHtml(cond.RoadCondition)}</span>
          </div>
          <div class="road-condition-badge">
            <span class="road-condition-badge-label">Weather</span>
            <span class="road-condition-badge-value" data-condition="${this.escapeHtml(cond.WeatherCondition)}">${this.escapeHtml(cond.WeatherCondition)}</span>
          </div>
          <div class="road-condition-badge${cond.Restriction !== 'none' ? ' road-condition-badge-warning' : ''}">
            <span class="road-condition-badge-label">Restriction</span>
            <span class="road-condition-badge-value">${this.escapeHtml(cond.Restriction)}</span>
          </div>
          <time class="road-condition-updated" datetime="${cond.LastUpdated}" data-last-updated="${cond.LastUpdated}">
            (<span class="road-condition-time-ago">${this.escapeHtml(this.formatTimeAgo(cond.LastUpdated))}</span> ago)
          </time>
        `;
        banner.appendChild(card);
      } else {
        // Update existing card
        const title = card.querySelector('.road-condition-title');
        const badges = card.querySelectorAll('.road-condition-badge');
        const roadBadge = badges[0];
        const weatherBadge = badges[1];
        const restrictionBadge = badges[2];
        
        const roadValue = roadBadge?.querySelector('.road-condition-badge-value');
        const weatherValue = weatherBadge?.querySelector('.road-condition-badge-value');
        const restrictionValue = restrictionBadge?.querySelector('.road-condition-badge-value');

        if (title && title.textContent !== cond.RoadwayName) {
          title.textContent = cond.RoadwayName;
        }
        if (roadValue && roadValue.textContent !== cond.RoadCondition) {
          roadValue.textContent = cond.RoadCondition;
          roadValue.setAttribute('data-condition', cond.RoadCondition);
        }
        if (weatherValue && weatherValue.textContent !== cond.WeatherCondition) {
          weatherValue.textContent = cond.WeatherCondition;
          weatherValue.setAttribute('data-condition', cond.WeatherCondition);
        }
        if (restrictionValue && restrictionValue.textContent !== cond.Restriction) {
          restrictionValue.textContent = cond.Restriction;
        }
        
        // Update restriction badge warning class
        if (restrictionBadge) {
          if (cond.Restriction !== 'none') {
            restrictionBadge.classList.add('road-condition-badge-warning');
          } else {
            restrictionBadge.classList.remove('road-condition-badge-warning');
          }
        }

        // Update timestamp
        let updatedTime = card.querySelector('.road-condition-updated');
        if (!updatedTime) {
          updatedTime = document.createElement('time');
          updatedTime.className = 'road-condition-updated';
          card.appendChild(updatedTime);
        }
        const currentTimestamp = parseInt(updatedTime.getAttribute('data-last-updated') || '0');
        if (currentTimestamp !== cond.LastUpdated) {
          updatedTime.setAttribute('datetime', cond.LastUpdated);
          updatedTime.setAttribute('data-last-updated', cond.LastUpdated);
          const timeAgoSpan = document.createElement('span');
          timeAgoSpan.className = 'road-condition-time-ago';
          timeAgoSpan.textContent = this.formatTimeAgo(cond.LastUpdated);
          updatedTime.textContent = '(';
          updatedTime.appendChild(timeAgoSpan);
          updatedTime.appendChild(document.createTextNode(' ago)'));
        } else {
          // Update relative time even if timestamp hasn't changed (for "X minutes ago" updates)
          const timeAgoSpan = updatedTime.querySelector('.road-condition-time-ago');
          if (timeAgoSpan) {
            timeAgoSpan.textContent = this.formatTimeAgo(cond.LastUpdated);
          }
        }
      }
    });
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
    if (!timestamp || timestamp === 0) {
      return 'unknown';
    }
    const now = Math.floor(Date.now() / 1000);
    const diff = now - timestamp;
    
    if (diff < 60) {
      return 'just now';
    } else if (diff < 3600) {
      const minutes = Math.floor(diff / 60);
      return `${minutes}m`;
    } else if (diff < 86400) {
      const hours = Math.floor(diff / 3600);
      return `${hours}h`;
    } else if (diff < 604800) {
      const days = Math.floor(diff / 86400);
      return `${days}d`;
    } else if (diff < 31536000) {
      const weeks = Math.floor(diff / 604800);
      return `${weeks}w`;
    } else {
      const years = Math.floor(diff / 31536000);
      return `${years}y`;
    }
  }

  startTimeAgoUpdates() {
    // Update all time-ago displays every minute
    const updateAllTimeAgo = () => {
      const banner = document.querySelector('.road-conditions-banner');
      if (!banner) return;

      banner.querySelectorAll('.road-condition-updated').forEach(updatedTime => {
        const timestamp = parseInt(updatedTime.getAttribute('data-last-updated') || '0');
        if (timestamp > 0) {
          const timeAgoSpan = updatedTime.querySelector('.road-condition-time-ago');
          if (timeAgoSpan) {
            timeAgoSpan.textContent = this.formatTimeAgo(timestamp);
          }
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
