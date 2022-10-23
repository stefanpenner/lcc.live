#!/usr/bin/env node

import express from "express";
import { URL } from "url";
import compression from "compression";
import { fetchBuilder, FileSystemCache } from "node-fetch-cache";

const fetch = fetchBuilder.withCache(
  new FileSystemCache({
    ttl: 30_000,
  })
);

const __dirname = new URL(".", import.meta.url).pathname;
const PORT = process.env.PORT || 8080;

const RELAY = /^[a-z][0-9]$/i;
const ID = /^[a-z0-9-]+$/i;
let i = 0;

export default function server() {
  return express()
    .use(compression())
    .get('/', (_, res) => res.sendFile(`${__dirname}/index.html`))
    .get("/hdrelay/:relay/:id", async (req, res) => {
      const { relay, id } = req.params;

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
        const response = await fetch(url);
        console.timeEnd(label);

        response.body.pipe(res);
      } catch (e) {
        console.error(e);
      }
    })
    .use(express.static(`${__dirname}/public`))
    .listen(PORT, () => console.log(`Listening on http://localhost:${PORT}`));
}
