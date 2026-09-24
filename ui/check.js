import { execFileSync } from "node:child_process";
import { before } from "./cooldown.js";

const run = (cmd, ...args) =>
  execFileSync(cmd, args, {
    cwd: new URL(".", import.meta.url),
    encoding: "utf8",
    stdio: ["ignore", "pipe", "inherit"],
  });

run("npm", "run", "--silent", "vendor");
if (run("git", "status", "--porcelain", "--", "../public/vendor") !== "") {
  console.error(
    "public/vendor/ doesn't match package-lock.json: run 'npm run vendor' in ui/ and commit the result.",
  );
  process.exit(1);
}

try {
  run("npm", "outdated", `--before=${before}`);
} catch (e) {
  console.error(
    `${e.stdout}\nDependencies are outdated: run 'npm run upgrade' in ui/ and commit the result.`,
  );
  process.exit(1);
}
