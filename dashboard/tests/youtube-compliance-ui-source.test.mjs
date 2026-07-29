import assert from "node:assert/strict";
import { existsSync, readdirSync, statSync } from "node:fs";
import { readdir, readFile } from "node:fs/promises";
import { basename, dirname, extname, join, relative, resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

const root = process.cwd();
const sourceRoot = resolve("src");
const publicRoot = resolve("public");
const explicitlyPublicAppRoots = new Set([
  "(platforms)",
  "about",
  "alternatives",
  "blog",
  "changelog",
  "compare",
  "connect",
  "contact",
  "docs",
  "marketing",
  "preview",
  "pricing",
  "privacy",
  "resources",
  "social-media-api",
  "social-media-posting-api",
  "social-media-publishing-api",
  "solutions",
  "terms",
  "tools",
]);
const nonUiAppRoots = new Set(["api"]);
const authenticatedRoots = readdirSync(resolve("src/app"), { withFileTypes: true })
  .filter((entry) => (
    entry.isDirectory()
    && !explicitlyPublicAppRoots.has(entry.name)
    && !nonUiAppRoots.has(entry.name)
  ))
  .map((entry) => resolve("src/app", entry.name));
const platformIconDefinition = resolve("src/components/platform-icons.tsx");
const publicYouTubeBrandIconDefinition = resolve("src/components/public/youtube-brand-icon.tsx");
const rootLayoutDefinition = resolve("src/app/layout.tsx");
const resolvableExtensions = [".ts", ".tsx", ".mts", ".js", ".jsx", ".mjs", ".css"];
const staticAssetExtensions = [".svg", ".png", ".webp"];
const dependencyExtensions = [...resolvableExtensions, ...staticAssetExtensions];
const allowedDynamicPlatformIconConsumers = new Set([
  "src/components/account-destination-icon.tsx", // rejects YouTube before delegating
  "src/components/analytics/meta-platform-analytics-view.tsx", // type-constrained to Instagram/Threads
]);
const allowedFixedPlatformIconConsumers = new Set([
  "src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx",
  "src/components/analytics/facebook-page-analytics-view.tsx",
  "src/components/analytics/pinterest-analytics-view.tsx",
  "src/components/analytics/tiktok-analytics-view.tsx",
]);
const allowedPlatformIconConsumers = new Set([
  ...allowedDynamicPlatformIconConsumers,
  ...allowedFixedPlatformIconConsumers,
]);

async function source(path) {
  return readFile(resolve(path), "utf8");
}

async function codeFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return codeFiles(path);
    return resolvableExtensions.includes(extname(entry.name)) ? [path] : [];
  }));
  return nested.flat();
}

async function staticAssetFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return staticAssetFiles(path);
    return staticAssetExtensions.includes(extname(entry.name)) ? [path] : [];
  }));
  return nested.flat();
}

function repositoryPath(path) {
  return relative(root, path).split("\\").join("/");
}

function isInsideSource(path) {
  const sourceRelativePath = relative(sourceRoot, path);
  return sourceRelativePath !== "" && !sourceRelativePath.startsWith("..") && !sourceRelativePath.startsWith("/");
}

function scriptKind(path) {
  if (path.endsWith(".tsx")) return ts.ScriptKind.TSX;
  if (path.endsWith(".jsx")) return ts.ScriptKind.JSX;
  if (path.endsWith(".js") || path.endsWith(".mjs")) return ts.ScriptKind.JS;
  return ts.ScriptKind.TS;
}

function parseSource(path, contents) {
  return ts.createSourceFile(path, contents, ts.ScriptTarget.Latest, true, scriptKind(path));
}

function moduleSpecifiers(path, contents) {
  if (path.endsWith(".css") || staticAssetExtensions.includes(extname(path))) return [];
  const specifiers = new Set();
  const sourceFile = parseSource(path, contents);

  function visit(node) {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node))
      && node.moduleSpecifier
      && ts.isStringLiteral(node.moduleSpecifier)
    ) {
      specifiers.add(node.moduleSpecifier.text);
    }
    if (
      ts.isCallExpression(node)
      && node.arguments.length === 1
      && ts.isStringLiteral(node.arguments[0])
      && (node.expression.kind === ts.SyntaxKind.ImportKeyword
        || (ts.isIdentifier(node.expression) && node.expression.text === "require"))
    ) {
      specifiers.add(node.arguments[0].text);
    }
    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return [...specifiers];
}

function resolveLocalDependency(importer, specifier) {
  let base;
  if (specifier.startsWith("@/")) {
    base = resolve(sourceRoot, specifier.slice(2));
  } else if (specifier.startsWith(".")) {
    base = resolve(dirname(importer), specifier);
  } else {
    return null;
  }

  const candidates = [
    base,
    ...dependencyExtensions.map((extension) => `${base}${extension}`),
    ...dependencyExtensions.map((extension) => join(base, `index${extension}`)),
  ];
  const match = candidates.find((candidate) => (
    isInsideSource(candidate) && existsSync(candidate) && statSync(candidate).isFile()
  ));
  return match ? resolve(match) : null;
}

async function authenticatedDependencyFiles() {
  const entryFiles = [
    rootLayoutDefinition,
    ...(await Promise.all(authenticatedRoots.map(codeFiles))).flat(),
  ];
  const pending = [...entryFiles];
  const visited = new Set();

  while (pending.length > 0) {
    const path = pending.pop();
    if (!path || visited.has(path)) continue;
    visited.add(path);

    const contents = await readFile(path, "utf8");
    for (const specifier of moduleSpecifiers(path, contents)) {
      const dependency = resolveLocalDependency(path, specifier);
      if (dependency && !visited.has(dependency)) pending.push(dependency);
    }
  }

  return [...visited];
}

function importedPlatformIconBindings(path, contents) {
  const sourceFile = parseSource(path, contents);
  const bindings = [];

  for (const statement of sourceFile.statements) {
    if (!ts.isImportDeclaration(statement) || !ts.isStringLiteral(statement.moduleSpecifier)) continue;
    if (resolveLocalDependency(path, statement.moduleSpecifier.text) !== platformIconDefinition) continue;

    const importClause = statement.importClause;
    if (!importClause?.namedBindings) continue;
    if (ts.isNamespaceImport(importClause.namedBindings)) {
      bindings.push({ kind: "namespace", localName: importClause.namedBindings.name.text });
      continue;
    }
    for (const element of importClause.namedBindings.elements) {
      if ((element.propertyName?.text || element.name.text) === "PlatformIcon") {
        bindings.push({ kind: "named", localName: element.name.text });
      }
    }
  }

  return { bindings, sourceFile };
}

function platformIconElements(sourceFile, bindings) {
  const elements = [];

  function matchesTag(tagName, binding) {
    if (binding.kind === "named") return ts.isIdentifier(tagName) && tagName.text === binding.localName;
    return ts.isPropertyAccessExpression(tagName)
      && ts.isIdentifier(tagName.expression)
      && tagName.expression.text === binding.localName
      && tagName.name.text === "PlatformIcon";
  }

  function visit(node) {
    if (ts.isJsxSelfClosingElement(node) || ts.isJsxOpeningElement(node)) {
      if (bindings.some((binding) => matchesTag(node.tagName, binding))) elements.push(node);
    }
    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return elements;
}

function platformAttributeValue(element) {
  const attribute = element.attributes.properties.find(
    (property) => ts.isJsxAttribute(property) && property.name.text === "platform",
  );
  if (!attribute || !ts.isJsxAttribute(attribute) || !attribute.initializer) return null;
  if (ts.isStringLiteral(attribute.initializer)) return attribute.initializer.text;
  if (
    ts.isJsxExpression(attribute.initializer)
    && attribute.initializer.expression
    && ts.isStringLiteral(attribute.initializer.expression)
  ) {
    return attribute.initializer.expression.text;
  }
  return "dynamic";
}

function assertPlatformIconBoundary(path, contents) {
  const repoPath = repositoryPath(path);
  const { bindings, sourceFile } = importedPlatformIconBindings(path, contents);

  if (bindings.length === 0) {
    for (const statement of sourceFile.statements) {
      if (
        ts.isExportDeclaration(statement)
        && statement.moduleSpecifier
        && ts.isStringLiteral(statement.moduleSpecifier)
        && resolveLocalDependency(path, statement.moduleSpecifier.text) === platformIconDefinition
      ) {
        assert.fail(`${repoPath} re-exports PlatformIcon into the authenticated dependency graph`);
      }
    }
    return;
  }

  assert.ok(
    allowedPlatformIconConsumers.has(repoPath),
    `${repoPath} imports PlatformIcon but is not an approved authenticated consumer`,
  );

  const elements = platformIconElements(sourceFile, bindings);
  assert.ok(elements.length > 0, `${repoPath} imports PlatformIcon without a directly auditable JSX use`);
  for (const element of elements) {
    const platform = platformAttributeValue(element);
    assert.notEqual(platform, "youtube", `${repoPath} renders the official YouTube PlatformIcon`);
    if (platform === "dynamic" || platform === null) {
      assert.ok(
        allowedDynamicPlatformIconConsumers.has(repoPath),
        `${repoPath} has an unapproved dynamic PlatformIcon`,
      );
    } else {
      assert.ok(
        allowedFixedPlatformIconConsumers.has(repoPath),
        `${repoPath} has a fixed PlatformIcon but is not classified as a fixed consumer`,
      );
    }
  }
}

function assertNoOfficialYouTubeGraphics(path, contents) {
  const repoPath = repositoryPath(path);
  const inspectableContents = contents
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");
  const pureYouTubeRedPatterns = [
    /#(?:ff0000(?:[0-9a-f]{2})?|f00(?:[0-9a-f])?)\b/i,
    /\brgba?\(\s*255(?:\s*,\s*|\s+)0(?:\s*,\s*|\s+)0(?:\s*(?:,|\/)\s*(?:0|1|0?\.\d+|\d+%))?\s*\)/i,
    /\brgba?\(\s*100%(?:\s*,\s*|\s+)0%(?:\s*,\s*|\s+)0%(?:\s*(?:,|\/)\s*(?:0|1|0?\.\d+|\d+%))?\s*\)/i,
    /\bhsla?\(\s*(?:(?:0|360)(?:deg)?|1turn)(?:\s*,\s*|\s+)100%(?:\s*,\s*|\s+)50%(?:\s*(?:,|\/)\s*(?:0|1|0?\.\d+|\d+%))?\s*\)/i,
    /\b(?:fill|stroke|stopColor|color|backgroundColor)\s*(?:=|:)\s*(?:\{\s*)?["']?\s*red\b/i,
  ];
  for (const pattern of pureYouTubeRedPatterns) {
    assert.doesNotMatch(inspectableContents, pattern, `${repoPath} contains official YouTube red`);
  }
  assert.doesNotMatch(inspectableContents, /M23\.498\s+6\.186/i, `${repoPath} contains the legacy YouTube path`);
  assert.doesNotMatch(
    inspectableContents,
    /(?:^|["'/(])(?:youtube|yt)[\w.-]*\.(?:png|svg|webp)/i,
    `${repoPath} references a YouTube brand asset`,
  );
}

test("authenticated roots include onboarding, direct protected routes, and the shared root layout", async () => {
  assert.ok(authenticatedRoots.includes(resolve("src/app/(onboarding)")));
  assert.ok(authenticatedRoots.includes(resolve("src/app/invite")));
  assert.ok((await authenticatedDependencyFiles()).includes(rootLayoutDefinition));
});

test("the shared PlatformIcon module contains no official YouTube artwork", async () => {
  const contents = await readFile(platformIconDefinition, "utf8");

  assert.doesNotMatch(contents, /\byoutube\s*:/i);
  assertNoOfficialYouTubeGraphics(platformIconDefinition, contents);
  assert.ok(existsSync(publicYouTubeBrandIconDefinition), "the official mark belongs in the public-only module");
});

test("the public YouTube brand component is the only owner of the official path", async () => {
  const files = await codeFiles(sourceRoot);

  for (const path of files) {
    if (path === publicYouTubeBrandIconDefinition) continue;
    const contents = await readFile(path, "utf8");
    assert.doesNotMatch(
      contents,
      /M23\.498\s+6\.186/i,
      `${repositoryPath(path)} duplicates the official YouTube artwork`,
    );
  }

  const assets = [
    ...(await staticAssetFiles(sourceRoot)),
    ...(await staticAssetFiles(publicRoot)),
  ];
  for (const path of assets) {
    assert.doesNotMatch(
      basename(path),
      /^(?:youtube|yt)[\w.-]*\.(?:png|svg|webp)$/i,
      `${repositoryPath(path)} creates a second YouTube brand asset owner`,
    );
    if (extname(path) === ".svg") {
      const contents = await readFile(path, "utf8");
      assert.doesNotMatch(
        contents,
        /M23\.498\s+6\.186/i,
        `${repositoryPath(path)} duplicates the official YouTube artwork`,
      );
    }
  }
});

test("the brand guard recognizes canonical representations of pure YouTube red", () => {
  for (const [name, contents] of [
    ["rgb comma", 'fill="rgb(255, 0, 0)"'],
    ["rgb spaces", 'fill="rgb(255 0 0)"'],
    ["rgba", 'fill="rgba(255, 0, 0, 0.8)"'],
    ["hex alpha", 'fill="#ff0000ff"'],
    ["rgb percentages", 'fill="rgb(100% 0% 0%)"'],
    ["hsl comma", 'fill="hsl(0, 100%, 50%)"'],
    ["hsl spaces", 'fill="hsl(0 100% 50%)"'],
    ["hsl full turn", 'fill="hsl(360 100% 50%)"'],
    ["keyword", 'fill="red"'],
  ]) {
    assert.throws(
      () => assertNoOfficialYouTubeGraphics(resolve(`src/app/(dashboard)/${name}.tsx`), contents),
      /official YouTube red/,
      name,
    );
  }

  assert.doesNotThrow(() => (
    assertNoOfficialYouTubeGraphics(resolve("src/app/(dashboard)/semantic-error.tsx"), 'color: "#ef4444"')
  ));
});

test("the brand guard rejects YouTube-named static image references", () => {
  assert.throws(
    () => assertNoOfficialYouTubeGraphics(
      resolve("src/app/(dashboard)/static-asset.tsx"),
      '<img src="/youtube.svg" alt="" />',
    ),
    /YouTube brand asset/,
  );
});

test("authenticated Dashboard dependency graph defaults to no official YouTube graphics", async () => {
  const files = await authenticatedDependencyFiles();

  for (const path of files) {
    const contents = await readFile(path, "utf8");

    assertNoOfficialYouTubeGraphics(path, contents);
    assertPlatformIconBoundary(path, contents);
  }

  assert.ok(
    !files.some((path) => repositoryPath(path) === "src/components/tools/ToolCard.tsx"),
    "the public ToolCard must not enter the authenticated dependency graph",
  );
  assert.ok(
    !files.includes(publicYouTubeBrandIconDefinition),
    "the public YouTube brand module must not enter the authenticated dependency graph",
  );
});

test("the authenticated boundary recognizes aliased and namespace PlatformIcon imports", () => {
  const aliasSource = 'import { PlatformIcon as BrandMark } from "@/components/platform-icons";';
  const namespaceSource = 'import * as BrandIcons from "@/components/platform-icons";';
  const fixturePath = resolve("src/app/(dashboard)/alias-guard-fixture.tsx");

  assert.deepEqual(importedPlatformIconBindings(fixturePath, aliasSource).bindings, [
    { kind: "named", localName: "BrandMark" },
  ]);
  assert.deepEqual(importedPlatformIconBindings(fixturePath, namespaceSource).bindings, [
    { kind: "namespace", localName: "BrandIcons" },
  ]);
  assert.throws(
    () => assertPlatformIconBoundary(
      fixturePath,
      `${aliasSource}\nexport const Fixture = () => <BrandMark platform="youtube" />;`,
    ),
    /not an approved authenticated consumer/,
  );
});

test("the public YouTube brand module is rejected by the authenticated graphics guard", async () => {
  const brandSource = await readFile(publicYouTubeBrandIconDefinition, "utf8");
  assert.throws(
    () => assertNoOfficialYouTubeGraphics(publicYouTubeBrandIconDefinition, brandSource),
    /official YouTube red/,
  );
});

test("the text source component owns validation and never embeds brand artwork", async () => {
  const linkSource = await source("src/components/youtube/youtube-source-link.tsx");

  assert.match(linkSource, /normalizeYouTubeContentUrl/);
  assert.match(linkSource, /<a/);
  assert.match(linkSource, /target="_blank"/);
  assert.match(linkSource, /rel="noopener noreferrer"/);
  assert.match(linkSource, /aria-label=\{accessibleLabel\}/);
  assert.match(linkSource, /Source: YouTube/);
  assert.match(linkSource, /Data from YouTube/);
  assert.match(linkSource, /View on YouTube/);
  assert.doesNotMatch(linkSource, /<img|<svg|#(?:ff0000|f00)|M23\.498\s+6\.186/i);
});

test("known operational surfaces use the neutral destination component", async () => {
  const paths = [
    "src/components/posts/create-post/account-card-grid.tsx",
    "src/components/posts/create-post/platform-editor-block.tsx",
    "src/components/posts/create-post/account-card.tsx",
    "src/components/dashboard/create-post-modal.tsx",
    "src/components/dashboard/connection-stats.tsx",
    "src/app/(dashboard)/projects/[id]/accounts/page.tsx",
    "src/app/(dashboard)/projects/[id]/inbox/page.tsx",
    "src/app/admin/users/page.tsx",
  ];

  for (const path of paths) {
    const contents = await source(path);
    assert.match(contents, /AccountDestinationIcon/, `${path} must use the neutral destination component`);
  }

  const accountsSource = await source("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
  const statsSource = await source("src/components/dashboard/connection-stats.tsx");
  const drawerSource = await source("src/components/posts/create-post/create-post-drawer.tsx");
  assert.match(accountsSource, /YouTubeChannelIdentity/);
  assert.match(accountsSource, /Source platform/);
  assert.match(accountsSource, /connected through UniPost-managed OAuth/);
  assert.match(statsSource, /UniPost-managed account health/);
  assert.match(drawerSource, /calculated by UniPost, not YouTube/);
});
