# base node image
FROM node:18-bullseye-slim as base
ADD etc/litefs.yml /etc/litefs.yml

COPY --from=litefs /usr/local/bin/litefs /usr/local/bin/litefs

# Install openssl for Prisma
RUN apt-get update && apt-get install -y openssl fuse

# Install all node_modules, including dev dependencies
FROM base as deps

RUN mkdir /app
WORKDIR /app

ADD package.json package-lock.json ./
RUN npm install --production=false

# Setup production node_modules
FROM base as production-deps

RUN mkdir /app
WORKDIR /app

COPY --from=deps /app/node_modules /app/node_modules
ADD package.json package-lock.json ./
RUN npm prune --production

# Build the app
FROM base as build

ENV NODE_ENV=production

RUN mkdir /app
WORKDIR /app

COPY --from=deps /app/node_modules /app/node_modules

# If we're using Prisma, uncomment to cache the prisma schema
# ADD prisma .
# RUN npx prisma generate

ADD . .
RUN npm run build

# Finally, build the production image with minimal footprint
FROM base

ENV NODE_ENV=production

RUN mkdir /app
WORKDIR /app

COPY --from=production-deps /app/node_modules /app/node_modules

# Uncomment if using Prisma
# COPY --from=build /app/node_modules/.prisma /app/node_modules/.prisma

COPY --from=build /app/build /app/build
COPY --from=build /app/public /app/public
ADD . .

<<<<<<< HEAD
<<<<<<< HEAD
COPY --from=litefs /usr/local/bin/litefs /usr/local/bin/litefs
ADD other/litefs.yml /etc/litefs.yml
RUN mkdir -p /data ${FLY_LITEFS_DIR}
CMD ["litefs", "mount", "--", "npm", "run", "start"]
=======
RUN mkdir -p /data /mnt/data

ENTRYPOINT ["litefs"]
CMD ["litefs", "mount", "--", "npm", "run", "setup", "start"]
>>>>>>> d1174d0 (litefs-sqlite)
=======
ENTRYPOINT "litefs"
>>>>>>> 854bfc4 (test)
