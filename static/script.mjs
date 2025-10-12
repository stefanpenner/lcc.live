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
      if (e.target === this.overlay || e.target.tagName === 'IMG' || e.target.tagName === 'IFRAME') {
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
      const img = e.target.closest('img');
      const iframe = e.target.closest('iframe');
      
      if (img && !img.closest('the-overlay')) {
        e.preventDefault();
        this.open(img);
      } else if (iframe && !iframe.closest('the-overlay')) {
        e.preventDefault();
        this.open(iframe);
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
    // Get all images and iframes in the page
    const images = Array.from(document.querySelectorAll('img')).filter(
      i => !i.closest('the-overlay')
    );
    const iframes = Array.from(document.querySelectorAll('iframe')).filter(
      i => !i.closest('the-overlay')
    );
    
    // Combine and sort by DOM order
    this.items = [...images, ...iframes].sort((a, b) => {
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
    
    // Clone and display element (image or iframe)
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
// Initialize
// ========================================

document.addEventListener('DOMContentLoaded', () => {
  const reloader = new ImageReloader(3000);
  reloader.start();

  const viewer = new FullscreenViewer();
});
