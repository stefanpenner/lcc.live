## lcc.live's codebase

[https://lcc.live](https://lcc.live/)

**Note:**  
- This is largely a hack and a playground—proceed with caution ("here be dragons").

**Overview:**

While this project might seem largely experimental, it's a fun place to explore different technologies and approaches. The goal is to provide [https://lcc.live](https://lcc.live/), a site that quickly displays all the webcams I CARE ABOUT covering Little and Big Cottonwood Canyons.

**TL;DR**
* single executable that in production runs within a tiny alpine linux image (7.8MB total)
* everything is served from memory
  * images are served from a custom store
  * static assets (html, css, js etc) are served from embed_fs
* 1 go-routine for fetching remote images, and updating the in memory store
* [echo](https://echo.labstack.com/) provides the server API
* running siege & doing some testing, it's only bottle-neck so far appears to be outbound IO.


**Current Status:**

- The current iteration is an experiment built using Go.
- Despite being somewhat rough around the edges, it performs exceptionally well and consumes few resources.
- The codebase may be messy, but it's surprisingly approachable, especially for someone new to Go.

---

Feel free to explore, experiment & contribute. PRs welcome!
