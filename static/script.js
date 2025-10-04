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
    
    // Click anywhere to close (including the image)
    this.addEventListener("click", () => this.hide());
  }

  empty() {
    clearTimeout(this.timer);
    [...this.childNodes].forEach((x) => x.remove());
  }

  hide() {
    this.style.display = "none";
    this.empty();
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

const maxWidth = window.matchMedia("(max-width: 724px)");

maxWidth.addEventListener("change",
  (e) => e.matches && document.querySelector("the-overlay").hide()
);

function findCamera(target) {
  return target.closest("camera-feed");
}

document.body.addEventListener("click", (e) => {
  let camera;
  if (maxWidth.matches === false && (camera = findCamera(e.target))) {
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
