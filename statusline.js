// Minimal Node.js wrapper: reads stdin from CC, pipes to cx.exe.
// Required on Windows because CC (Node.js) -> native .exe stdin pipe is broken.
const { execFileSync } = require("child_process");
const { join } = require("path");

const stdin = require("fs").readFileSync(0, "utf8");
const cx = join(__dirname, process.platform === "win32" ? "cx.exe" : "cx");

try {
  const out = execFileSync(cx, ["statusline"], { input: stdin, encoding: "utf8" });
  process.stdout.write(out);
} catch {
  process.stdout.write("[?]");
}
