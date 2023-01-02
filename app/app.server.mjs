import { PrismaClient } from "@prisma/client";
const prisma = new PrismaClient();

export default class App {
  static #app = new App(prisma);
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
