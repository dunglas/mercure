import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { before } from "./cooldown.js";

const { dependencies, devDependencies } = JSON.parse(
  await readFile(new URL("package.json", import.meta.url)),
);
const latest = (deps) => Object.keys(deps).map((name) => `${name}@latest`);
const npm = (...args) =>
  execFileSync("npm", args, {
    cwd: new URL(".", import.meta.url),
    stdio: "inherit",
  });

const install = [
  "install",
  "--save-exact",
  "--ignore-scripts",
  `--before=${before}`,
];
npm(...install, ...latest(dependencies));
npm(...install, "--save-dev", ...latest(devDependencies));
npm("run", "vendor");
