export type BlogInlineSegment =
  | { type: "text"; text: string }
  | { type: "strong"; text: string }
  | { type: "link"; text: string; href: string }
  | { type: "code"; text: string };

const inlinePattern = /\[([^\]]+)\]\(([^)]+)\)|`([^`]+)`|\*\*([^*\n]+)\*\*/g;

export function parseInlineMarkdown(input: string): BlogInlineSegment[] {
  const segments: BlogInlineSegment[] = [];
  let last = 0;
  let match: RegExpExecArray | null;

  while ((match = inlinePattern.exec(input)) !== null) {
    if (match.index > last) {
      appendText(segments, input.slice(last, match.index));
    }

    if (match[1] && match[2]) {
      segments.push({ type: "link", text: match[1], href: match[2] });
    } else if (match[3]) {
      segments.push({ type: "code", text: match[3] });
    } else if (match[4]) {
      segments.push({ type: "strong", text: match[4] });
    }

    last = inlinePattern.lastIndex;
  }

  if (last < input.length) {
    appendText(segments, input.slice(last));
  }

  return segments.length > 0 ? segments : [{ type: "text", text: input }];
}

function appendText(segments: BlogInlineSegment[], text: string) {
  if (!text) {
    return;
  }
  const previous = segments.at(-1);
  if (previous?.type === "text") {
    previous.text += text;
    return;
  }
  segments.push({ type: "text", text });
}
