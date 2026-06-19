export function splitDocsAiAnswerParagraphs(answerText: string) {
  const explicitParagraphs = answerText
    .split(/\n{2,}/)
    .map((paragraph) => paragraph.replace(/\s+/g, " ").trim())
    .filter(Boolean);

  if (explicitParagraphs.length > 1) return explicitParagraphs;

  const normalized = answerText.replace(/\s+/g, " ").trim();
  if (!normalized) return [];

  const sentences = normalized
    .split(/(?<=[.!?])\s+/)
    .map((sentence) => sentence.trim())
    .filter(Boolean);
  const atomicParts = sentences.length > 1
    ? sentences
    : normalized
      .split(/;\s+(?=(?:then|call|use|redirect|store|if|create|send|read)\b)/i)
      .map((part, index, parts) => index < parts.length - 1 ? `${part};` : part)
      .map((part) => part.trim())
      .filter(Boolean);

  if (sentences.length <= 1 && atomicParts.length > 1) return atomicParts;

  const paragraphs: string[] = [];
  let current = "";

  for (const part of atomicParts) {
    const next = current ? `${current} ${part}` : part;
    if (current && next.length > 230) {
      paragraphs.push(current);
      current = part;
      continue;
    }
    current = next;
  }

  if (current) paragraphs.push(current);
  return paragraphs;
}
