#!/usr/bin/env node
import repl from "node:repl";
import App from "./app/app.server.mjs";
import * as queries from "./app/db.server.mjs";

// TODO: fully implement more advanced features provided by https://nodejs.org/api/repl.html
App.app.queries = queries;
const server = repl.start("lcc.live ➜ ");
server.context.app = App.app;
server.setupHistory(process.cwd() + ".repl-history", (repl) => {});
