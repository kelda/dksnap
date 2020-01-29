FROM node:10-alpine
ENV PORT 8080
WORKDIR /usr/src/app
COPY . /usr/src/app

RUN ["npm", "install"]

ENTRYPOINT ["node", "/usr/src/app/server.js"]
