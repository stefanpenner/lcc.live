import { json } from "@remix-run/node";
import {
  LiveReload,
  Outlet,
  Scripts,
  ScrollRestoration,
  NavLink,
  useLocation,
} from "@remix-run/react";
import {
  useEffect
} from "react";

import style from "./styles/main.css";

export default function App() {
  let roadStatus;
  let location = useLocation();

  switch (location.pathname.toLowerCase()) {
    case '/': {
      roadStatus = <road-status>
        <img src="https://www.udottraffic.utah.gov/AnimatedGifs/100032.gif" alt="210 highway status" />
      </road-status>
      break;
    } 
    case '/bcc': {
      roadStatus = <road-status>
        <img src="http://www.udottraffic.utah.gov/AnimatedGifs/100033.gif" alt="SR-190 highway status" />
      </road-status>
    }
  }

  if (typeof window === 'object' && !Array.isArray(window.dataLayer)) {
    const d = window.dataLayer = window.dataLayer || [];
    d.push(["js", new Date()]);
    d.push(["config", "UA-31100913-2"]);
  }

  useEffect(() => {
    const d = window.dataLayer;
    if (Array.isArray(d)){
      // Google Analytics
      d.push(['send', 'pageView']);
    }
  }, [location]);

  return (
    <html lang="en">
      <head>
        <script
          async
          src="https://www.googletagmanager.com/gtag/js?id=UA-31100913-2"
        ></script>
        <title>[LIVE] - LCC</title>
        <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover" />
        <meta name="charset" content="utf-8" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
        <meta name="apple-mobile-web-app-status-bar-style" content="white" />
        <meta name="theme-color" content="#f2f3f4" />
        <meta
          name="msapplication-square310x310logo"
          content="icon_largetile.png"
        />
        <meta name="Description" content="LCC Live" />
        <link rel="icon" sizes="192x192" href="icon.png" />
        <link rel="apple-touch-icon" href="ios-icon.png" />
        <link rel="manifest" href="/manifest.json" />
        <link rel="stylesheet" type="text/css" href={style} />
      </head>
      <body>
        <header>
          <nav className="canyon-toggle">
            <NavLink prefetch="intent" to="/">
              LCC
            </NavLink>
            <NavLink prefetch="intent" to="/bcc">
              BCC
            </NavLink>
          </nav>
          {roadStatus}
        </header>
        <Outlet />
        <ScrollRestoration />
        <Scripts />
        <LiveReload />

        <the-overlay></the-overlay>
        <script src="/script.js"></script>
      </body>
    </html>
  );
}
