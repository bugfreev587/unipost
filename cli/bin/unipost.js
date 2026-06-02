#!/usr/bin/env node
import { main } from "../src/cli.js";

main(process.argv.slice(2), {
  env: process.env,
  stdout: process.stdout,
  stderr: process.stderr,
  fetchImpl: globalThis.fetch,
}).then((exitCode) => {
  process.exitCode = exitCode;
}).catch((error) => {
  process.stderr.write(`${error?.message || "Unexpected error"}\n`);
  process.exitCode = 1;
});
