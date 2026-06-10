const http = require("http");
const fs = require("fs");
const data = JSON.parse(fs.readFileSync(__dirname + "/src/lib/__fixtures__/video-feed-response.json", "utf8"));
const server = http.createServer((req, res) => {
  res.setHeader("Content-Type", "application/json");
  const url = req.url.split("?")[0];
  if (url === "/public/v1/videos") {
    res.end(JSON.stringify(data));
    return;
  }
  const m = url.match(/^\/public\/v1\/videos\/(.+)$/);
  if (m) {
    const id = decodeURIComponent(m[1]);
    const item = data.items.find((i) => i.id === id);
    if (item) { res.end(JSON.stringify(item)); return; }
    res.statusCode = 404; res.end(JSON.stringify({ error: "not found" })); return;
  }
  res.statusCode = 404; res.end(JSON.stringify({ error: "unknown" }));
});
server.listen(8799, () => console.log("mock api on 8799"));
