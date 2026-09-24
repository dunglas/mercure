import { copyFile, mkdir, readFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { build } from "esbuild";

const outdir = new URL("../public/vendor/", import.meta.url).pathname;
const require = createRequire(import.meta.url);
const { dependencies } = JSON.parse(
  await readFile(new URL("package.json", import.meta.url)),
);

await rm(outdir, { recursive: true, force: true });

await build({
  entryPoints: { "fetch-event-source": "@microsoft/fetch-event-source" },
  absWorkingDir: dirname(new URL(import.meta.url).pathname),
  bundle: true,
  format: "esm",
  outdir,
  logLevel: "warning",
});

// The bundles keep no license header: ship each package's license next to them.
await mkdir(join(outdir, "licenses"));
for (const name of Object.keys(dependencies)) {
  const root = dirname(require.resolve(`${name}/package.json`));
  await copyFile(
    join(root, "LICENSE"),
    join(outdir, "licenses", `${name.replace("@", "").replace("/", "_")}.txt`),
  );
}
