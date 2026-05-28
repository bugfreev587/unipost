import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { resolveChromiumWindowBounds, startNativeBrowserCapture } from "../src/native-capture.js";

test("resolveChromiumWindowBounds uses CDP to size the visible browser window", async () => {
  const sends = [];
  let getCount = 0;
  const session = {
    send: async (method, payload) => {
      sends.push({ method, payload });
      if (method === "Browser.getWindowForTarget") {
        getCount += 1;
        if (getCount > 1) {
          return { windowId: 7, bounds: { left: 120, top: 90, width: 1280, height: 720 } };
        }
        return { windowId: 7, bounds: { left: 12, top: 24, width: 900, height: 700 } };
      }
      return {};
    },
  };
  const page = { context: () => ({ newCDPSession: async () => session }) };

  const bounds = await resolveChromiumWindowBounds({ page, recording: { window_left: 120, window_top: 90, window_width: 1280, window_height: 720 } });

  assert.deepEqual(bounds, { left: 120, top: 90, width: 1280, height: 720 });
  assert.equal(sends[0].method, "Browser.getWindowForTarget");
  assert.deepEqual(sends[1], {
    method: "Browser.setWindowBounds",
    payload: { windowId: 7, bounds: { left: 120, top: 90, width: 1280, height: 720, windowState: "normal" } },
  });
});

test("resolveChromiumWindowBounds returns actual visible bounds after the OS clamps the requested window", async () => {
  const sends = [];
  let getCount = 0;
  const session = {
    send: async (method, payload) => {
      sends.push({ method, payload });
      if (method === "Browser.getWindowForTarget") {
        getCount += 1;
        if (getCount === 1) {
          return { windowId: 7, bounds: { left: 12, top: 24, width: 900, height: 700 } };
        }
        return { windowId: 7, bounds: { left: 0, top: 38, width: 1728, height: 1030 } };
      }
      return {};
    },
  };
  const page = { context: () => ({ newCDPSession: async () => session }) };

  const bounds = await resolveChromiumWindowBounds({
    page,
    recording: { window_left: 0, window_top: 0, window_width: 1920, window_height: 1080 },
  });

  assert.deepEqual(bounds, { left: 0, top: 38, width: 1728, height: 1030 });
  assert.deepEqual(sends.map((call) => call.method), [
    "Browser.getWindowForTarget",
    "Browser.setWindowBounds",
    "Browser.getWindowForTarget",
  ]);
});

test("startNativeBrowserCapture records a macOS browser-window rectangle", async () => {
  const proc = new EventEmitter();
  proc.stderr = new EventEmitter();
  proc.kill = (signal) => process.nextTick(() => proc.emit("exit", null, signal));
  proc.off = proc.removeListener.bind(proc);
  let spawnCommand = "";
  let spawnArgs = [];
  const spawnImpl = (command, args) => {
    spawnCommand = command;
    spawnArgs = args;
    return proc;
  };
  const session = {
    send: async (method) => method === "Browser.getWindowForTarget" ? { windowId: 3, bounds: { width: 1000, height: 800 } } : {},
  };
  const page = { context: () => ({ newCDPSession: async () => session }) };

  const capture = await startNativeBrowserCapture({
    script: {
      job_id: "rvjob_native",
      recording: { show_address_bar: true, capture_mode: "native-browser-window", window_width: 1280, window_height: 720 },
    },
    page,
    browser: {},
    outputDir: "/tmp/unipost-review-native-test",
    platform: "darwin",
    spawnImpl,
    startDelayMs: 0,
    out: { write() {} },
  });

  assert.equal(spawnCommand, "screencapture");
  assert.deepEqual(spawnArgs.slice(0, 4), ["-v", "-x", "-R", "80,80,1000,800"]);
  assert.equal(spawnArgs[4], "/tmp/unipost-review-native-test/rvjob_native-browser-window.mov");
  assert.equal(capture.includesAddressBar, true);
  await capture.stop();
});
