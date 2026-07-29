import assert from "node:assert/strict";
import { existsSync, statSync } from "node:fs";
import { readdir, readFile } from "node:fs/promises";
import { dirname, extname, join, relative, resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

const root = process.cwd();
const sourceRoot = resolve("src");
const authenticatedRoots = [
  resolve("src/app/(dashboard)"),
  resolve("src/app/admin"),
];
const platformIconDefinition = resolve("src/components/platform-icons.tsx");
const resolvableExtensions = [".ts", ".tsx", ".mts", ".js", ".jsx", ".mjs", ".css"];
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
  if (path.endsWith(".css")) return [];
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
    ...resolvableExtensions.map((extension) => `${base}${extension}`),
    ...resolvableExtensions.map((extension) => join(base, `index${extension}`)),
  ];
  const match = candidates.find((candidate) => (
    isInsideSource(candidate) && existsSync(candidate) && statSync(candidate).isFile()
  ));
  return match ? resolve(match) : null;
}

async function authenticatedDependencyFiles() {
  const entryFiles = (await Promise.all(authenticatedRoots.map(codeFiles))).flat();
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

test("authenticated Dashboard dependency graph defaults to no official YouTube graphics", async () => {
  const files = await authenticatedDependencyFiles();

  for (const path of files) {
    if (path === platformIconDefinition) continue;
    const repoPath = repositoryPath(path);
    const contents = await readFile(path, "utf8");

    assert.doesNotMatch(contents, /#(?:ff0000|f00)\b/i, `${repoPath} contains official YouTube red`);
    assert.doesNotMatch(contents, /M23\.498\s+6\.186/i, `${repoPath} contains the legacy YouTube path`);
    assert.doesNotMatch(
      contents,
      /(?:youtube|yt)[\w./-]*(?:icon|logo)[\w./-]*\.(?:png|svg|webp)/i,
      `${repoPath} references a YouTube brand asset`,
    );
    assertPlatformIconBoundary(path, contents);
  }

  assert.ok(
    !files.some((path) => repositoryPath(path) === "src/components/tools/ToolCard.tsx"),
    "the public ToolCard must not enter the authenticated dependency graph",
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

test("the public ToolCard becomes forbidden if it enters the authenticated dependency graph", async () => {
  const toolCardPath = resolve("src/components/tools/ToolCard.tsx");
  const toolCardSource = await readFile(toolCardPath, "utf8");
  assert.throws(
    () => assertPlatformIconBoundary(toolCardPath, toolCardSource),
    /not an approved authenticated consumer/,
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
