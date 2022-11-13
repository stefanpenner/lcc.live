import { PassThrough } from "stream";

import { Response } from "@remix-run/node";
import { RemixServer } from "@remix-run/react";
import isbot from "isbot";
import { renderToPipeableStream } from "react-dom/server";

const ABORT_DELAY = 5000;

import { fetchBuilder, FileSystemCache } from "node-fetch-cache";
let i = 0;

async function handleImage(request, responseStatusCode, responseHeaders, remixContext) {
  const RELAY = /^[a-z][0-9]$/i;
  const ID = /^[a-z0-9-]+$/i;

  const [,type, relay, id] =new URL(request.url).pathname.split('/')
  if (type !== 'hdrelay') {
    throw new Error('unsupported image cache type');
  }

  if (RELAY.test(relay) === false) {
    throw new Error(`invalid RELAY: ${relay}`);
  }
  if (ID.test(id) === false) {
    throw new Error(`invalid ID: ${id}`);
  }

  const url = `https://${relay}.hdrelay.com/camera/${id}/snapshot`;

  i++;
  const label = `[${i}] fetching: ${url}`;
  console.time(label);
  try {
    return fetch(url);
  } catch (e) {
    console.error(e);
  }
}

export default function handleRequest(
  request,
  responseStatusCode,
  responseHeaders,
  remixContext
) {
  const callbackName = isbot(request.headers.get("user-agent"))
    ? "onAllReady"
    : "onShellReady";

  const url = new URL(request.url);

  if (url.pathname.startsWith('/hdrelay')) {
    return handleImage(request, responseStatusCode, responseHeaders, remixContext);
  } else {
    return new Promise((resolve, reject) => {
      let didError = false;

      const { pipe, abort } = renderToPipeableStream(
        <RemixServer context={remixContext} url={request.url} />,
        {
          [callbackName]: () => {
            const body = new PassThrough();

            responseHeaders.set("Content-Type", "text/html");

            resolve(
              new Response(body, {
                headers: responseHeaders,
                status: didError ? 500 : responseStatusCode,
              })
            );

            pipe(body);
          },
          onShellError: (err) => {
            reject(err);
          },
          onError: (error) => {
            didError = true;

            console.error(error);
          },
        }
      );

      setTimeout(abort, ABORT_DELAY);
    });
  }
}
