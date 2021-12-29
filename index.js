#!/usr/bin/env node
const express = require('express')
const path = require('path')
const PORT = process.env.PORT || 8080

express()
  .use(express.static(`${__dirname}/public`))
  .listen(PORT, () => console.log(`Listening on http://localhost:${ PORT }`))
