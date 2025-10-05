// ========================================
// Theme Management
// ========================================
(function initTheme() {
  // Get stored theme or detect system preference
  const storedTheme = localStorage.getItem('theme');
  const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  
  // Apply theme immediately to avoid flash
  if (storedTheme) {
    document.documentElement.setAttribute('data-theme', storedTheme);
  } else if (systemPrefersDark) {
    document.documentElement.setAttribute('data-theme', 'dark');
  }
})();

// Theme toggle functionality
document.addEventListener('DOMContentLoaded', () => {
  const themeToggle = document.querySelector('.theme-toggle');
  
  if (themeToggle) {
    themeToggle.addEventListener('click', () => {
      const currentTheme = document.documentElement.getAttribute('data-theme');
      const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
      
      document.documentElement.setAttribute('data-theme', newTheme);
      localStorage.setItem('theme', newTheme);
      
      // Optional: announce to screen readers
      const announcement = `Switched to ${newTheme} mode`;
      themeToggle.setAttribute('aria-label', announcement);
      setTimeout(() => {
        themeToggle.setAttribute('aria-label', 'Toggle dark mode');
      }, 1000);
    });
  }
  
  // Listen for system theme changes
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    // Only auto-switch if user hasn't manually set a preference
    if (!localStorage.getItem('theme')) {
      document.documentElement.setAttribute('data-theme', e.matches ? 'dark' : 'light');
    }
  });
});

// ========================================
// Overlay Component
// ========================================
class Overlay extends HTMLElement {
  constructor() {
    super();
    this.currentIndex = 0;
    this.cameras = [];
    this.timer = null;
    
    // Touch handling for swipe gestures
    this.touchStartX = 0;
    this.touchEndX = 0;
    this.touchStartY = 0;
    this.touchEndY = 0;
    this.swiped = false;
    
    // Click on overlay or enlarged image to close
    this.addEventListener("click", (e) => {
      // Don't close if the user just swiped
      if (this.swiped) {
        this.swiped = false;
        return;
      }
      
      // Close overlay on click (anywhere on overlay or image)
      // This mimics standard lightbox behavior
      this.hide();
    });
    
    // Add touch event listeners for swipe gestures
    this.addEventListener("touchstart", (e) => this.handleTouchStart(e), { passive: true });
    this.addEventListener("touchend", (e) => this.handleTouchEnd(e), { passive: true });
  }
  
  handleTouchStart(e) {
    this.touchStartX = e.changedTouches[0].screenX;
    this.touchStartY = e.changedTouches[0].screenY;
  }
  
  handleTouchEnd(e) {
    this.touchEndX = e.changedTouches[0].screenX;
    this.touchEndY = e.changedTouches[0].screenY;
    this.handleSwipe();
  }
  
  handleSwipe() {
    const deltaX = this.touchEndX - this.touchStartX;
    const deltaY = this.touchEndY - this.touchStartY;
    const minSwipeDistance = 50; // Minimum distance in pixels to trigger swipe
    
    // Check for vertical swipe (down to close)
    if (Math.abs(deltaY) > Math.abs(deltaX) && Math.abs(deltaY) > minSwipeDistance) {
      if (deltaY > 0) {
        // Swiped down - close overlay
        this.swiped = true;
        this.hide();
        return;
      }
    }
    
    // Check for horizontal swipe (left/right to navigate)
    if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > minSwipeDistance) {
      this.swiped = true;
      if (deltaX > 0) {
        // Swiped right - go to previous image
        this.navigatePrevious();
      } else {
        // Swiped left - go to next image
        this.navigateNext();
      }
      
      // Reset swiped flag after a short delay
      setTimeout(() => {
        this.swiped = false;
      }, 300);
    }
  }

  empty() {
    clearTimeout(this.timer);
    [...this.childNodes].forEach((x) => x.remove());
  }

  hide() {
    this.style.display = "none";
    this.empty();
    // Restore scrolling
    document.body.style.overflow = "";
    // Restore focus to the camera that was opened
    if (this.currentIndex >= 0 && this.cameras[this.currentIndex]) {
      this.cameras[this.currentIndex].focus();
    }
  }

  reload() {
    this.timer = setTimeout(() => {
      const img = this.querySelector("img");
      if (img) {
        reloadImage(img);
        this.timer = setTimeout(() => this.reload(), 30_000);
      }
    }, 1_000);
  }

  show() {
    this.style.display = "flex";
    // Disable scrolling
    document.body.style.overflow = "hidden";
    this.reload();
  }

  showCamera(index) {
    if (index < 0 || index >= this.cameras.length) return;
    
    this.currentIndex = index;
    const camera = this.cameras[index];
    
    if (!camera.closest("body")) return;
    
    const cloned = camera.cloneNode(true);
    cloned.removeAttribute("tabindex");
    cloned.setAttribute("role", "img");
    
    // Add smooth fade-in
    cloned.style.opacity = "0";
    cloned.style.transition = "opacity 0.2s ease";

    this.empty();
    this.appendChild(cloned);
    this.show();
    
    // Detect image aspect ratio and apply appropriate sizing
    const img = cloned.querySelector("img");
    if (img) {
      // Wait for image to load to get dimensions
      if (img.complete) {
        this.applyImageSizing(img);
      } else {
        img.addEventListener("load", () => this.applyImageSizing(img));
      }
    }
    
    // Trigger fade-in
    requestAnimationFrame(() => {
      cloned.style.opacity = "1";
    });
  }

  applyImageSizing(img) {
    const aspectRatio = img.naturalWidth / img.naturalHeight;
    
    // Portrait: taller than wide
    if (aspectRatio < 0.9) {
      img.classList.add("portrait");
    } else {
      img.classList.remove("portrait");
    }
  }

  navigatePrevious() {
    if (this.currentIndex > 0) {
      this.showCamera(this.currentIndex - 1);
    }
  }

  navigateNext() {
    if (this.currentIndex < this.cameras.length - 1) {
      this.showCamera(this.currentIndex + 1);
    }
  }

  cameraWasClicked(camera) {
    // Update camera list on each click
    this.cameras = [...document.querySelectorAll("camera-feed")];
    const index = this.cameras.indexOf(camera);
    this.showCamera(index);
  }
}

customElements.define("the-overlay", Overlay);

document.addEventListener("keydown", (e) => {
  const overlay = document.querySelector("the-overlay");
  const isOverlayVisible = overlay.style.display === "flex" || overlay.style.display === "block";
  
  switch (e.key) {
    case "Escape": {
      if (isOverlayVisible) {
        overlay.hide();
        e.preventDefault();
      }
      break;
    }
    case "ArrowLeft":
    case "Left": {
      if (isOverlayVisible) {
        overlay.navigatePrevious();
        e.preventDefault();
      }
      break;
    }
    case "ArrowRight":
    case "Right": {
      if (isOverlayVisible) {
        overlay.navigateNext();
        e.preventDefault();
      }
      break;
    }
    case "Enter": {
      if (!isOverlayVisible) {
        const { activeElement } = document;
        if (activeElement.closest("camera-feed")) {
          overlay.cameraWasClicked(activeElement);
          e.preventDefault();
        }
      }
      break;
    }
  }
});

// Removed problematic mobile overlay hiding - fullscreen should work on all screen sizes

function findCamera(target) {
  return target.closest("camera-feed");
}

document.body.addEventListener("click", (e) => {
  // Don't open overlay if clicking share button
  if (e.target.closest('.share-button')) {
    return;
  }
  
  let camera;
  if (camera = findCamera(e.target)) {
    e.preventDefault(); // Prevent navigation to camera detail page
    document.querySelector("the-overlay").cameraWasClicked(camera);
  }
});

function forceReload(image) {
  image.src = image.src
}

const wait = async (time) =>
  new Promise((resolve) => setTimeout(resolve, time));

document.addEventListener("visibilitychange", (event) => {
  if (event.target.visibilityState !== "visible") {
    return;
  }
  document.querySelectorAll("img").forEach((image) => forceReload(image));
});

(async function reloadImages() {
  // todo: timer, abort controller etc.
  self.console && console.time && console.time("load images")
  await Promise.allSettled([...document.querySelectorAll("img")].map(reloadImage))
  self.console && console.timeEnd && console.timeEnd("load images")
  await wait(2_000);
  reloadImages();
})();

(async function reloadPage() {
  await wait(600_000 + (Math.random() * 1_200_000)); // Random wait between 5 - ~25 minutes
  self.location.reload()
})();

async function reloadImage(img) {
  try {
    img.dataset.src = img.dataset.src || img.src
   
    if (!img.classList.contains("in-viewport")) {
      return
    }

    const request = await fetch(img.dataset.src, {
      mode: 'same-origin',
      cache: 'default'
    });

    if (request.status === 200) {
      const etag = request.headers.get('etag')
      if (img.dataset.etag != etag) {
        img.dataset.etag = etag
        
        // Add smooth transition when updating image
        const oldSrc = img.src;
        const newBlob = await request.blob();
        const newUrl = URL.createObjectURL(newBlob);
        
        // Preload the image
        const tempImg = new Image();
        tempImg.onload = () => {
          img.style.opacity = '0.7';
          img.style.transition = 'opacity 0.3s ease';
          
          setTimeout(() => {
            img.src = newUrl;
            img.style.opacity = '1';
            
            // Clean up old blob URL
            if (oldSrc.startsWith('blob:')) {
              URL.revokeObjectURL(oldSrc);
            }
          }, 150);
        };
        tempImg.src = newUrl;
      }
    }
  } catch (error) {
    // Network error or fetch failed - silently fail and retry on next cycle
    console.warn('Failed to reload image:', img.dataset.src, error);
  }
}

const images = document.querySelectorAll('img');
const observer = new IntersectionObserver((entries) => {
  entries.forEach(entry => {
    if (entry.isIntersecting) {
      entry.target.classList.add('in-viewport');
    } else {
      entry.target.classList.remove('in-viewport');
    }
  });
}, {
    root: null,
    rootMargin: '0px',
    threshold: 0
  });

images.forEach(img => observer.observe(img));

// Subtle page load animations
document.addEventListener('DOMContentLoaded', () => {
  // Subtle fade in for camera feeds (only if user prefers motion)
  const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  
  if (!prefersReducedMotion) {
    const feeds = document.querySelectorAll('camera-feed');
    feeds.forEach((feed, index) => {
      feed.style.opacity = '0';
      feed.style.transition = 'opacity 0.25s ease';
      
      setTimeout(() => {
        feed.style.opacity = '1';
      }, 30 + (index * 20)); // Quick stagger
    });
  }
});

// ========================================
// Share Button Functionality
// ========================================
document.body.addEventListener('click', async (e) => {
  const shareButton = e.target.closest('.share-button');
  if (!shareButton) return;
  
  // Prevent camera click/overlay from opening
  e.stopPropagation();
  e.preventDefault();
  
  const cameraId = shareButton.dataset.cameraId;
  const cameraName = shareButton.dataset.cameraName;
  const cameraUrl = `${window.location.origin}/camera/${cameraId}`;
  
  // Try native Web Share API first (available on mobile)
  if (navigator.share) {
    try {
      await navigator.share({
        title: `${cameraName} Live Camera`,
        text: `Check out this live camera feed from ${cameraName}`,
        url: cameraUrl
      });
      return;
    } catch (err) {
      // User cancelled or share failed, fall back to copy
      if (err.name !== 'AbortError') {
        console.warn('Share failed:', err);
      }
    }
  }
  
  // Fallback: Copy to clipboard
  try {
    await navigator.clipboard.writeText(cameraUrl);
    
    // Visual feedback
    const originalHTML = shareButton.innerHTML;
    shareButton.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>
    `;
    shareButton.style.color = 'var(--accent-focus)';
    shareButton.style.opacity = '1';
    
    setTimeout(() => {
      shareButton.innerHTML = originalHTML;
      shareButton.style.color = '';
      shareButton.style.opacity = '';
    }, 2000);
  } catch (err) {
    console.error('Failed to copy to clipboard:', err);
    // If clipboard fails, try to select the URL in a temporary input
    const input = document.createElement('input');
    input.value = cameraUrl;
    document.body.appendChild(input);
    input.select();
    document.execCommand('copy');
    document.body.removeChild(input);
  }
});
