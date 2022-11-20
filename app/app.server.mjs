import { PrismaClient } from "@prisma/client";

export default class App {
  static #app = new App(new PrismaClient());
  static get app() {
    return this.#app;
  }

  #db;
  constructor(db) {
    this.#db = db;
  }

  get db() {
    return this.#db;
  }
}
