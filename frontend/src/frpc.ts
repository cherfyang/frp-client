import { parse } from 'smol-toml';

export interface RootEntry {
  key: string;
  value: unknown;
  kind: 'value' | 'table';
}

export interface ParsedSection {
  sectionKey: string;
  index: number;
  name: string;
  data: Record<string, unknown>;
  startLine: number;
  endLine: number;
  block: string;
}

export interface DuplicateName {
  sectionKey: string;
  name: string;
  count: number;
}

export interface ParsedFrpcDocument {
  rootEntries: RootEntry[];
  sections: ParsedSection[];
  sectionCounts: Record<string, number>;
  sectionKeys: string[];
  duplicateNames: DuplicateName[];
}

type SerializableValue = string | number | boolean | Array<string | number | boolean>;

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === 'object' && !Array.isArray(value) && !(value instanceof Date);

const isArrayTable = (value: unknown): value is Record<string, unknown>[] =>
  Array.isArray(value) && value.every((item) => isPlainObject(item));

export const formatValuePreview = (value: unknown): string => {
  if (Array.isArray(value)) {
    return `[${value.map((item) => formatValuePreview(item)).join(', ')}]`;
  }

  if (isPlainObject(value)) {
    return Object.entries(value)
      .map(([key, item]) => `${key}: ${formatValuePreview(item)}`)
      .join('\n');
  }

  if (value instanceof Date) {
    return value.toISOString();
  }

  if (typeof value === 'string') {
    return value;
  }

  return String(value);
};

export const parseFrpcDocument = (raw: string): ParsedFrpcDocument => {
  const parsed = parse(raw) as Record<string, unknown>;
  const sectionBlocks = collectSectionBlocks(raw);
  const rootEntries: RootEntry[] = [];
  const sections: ParsedSection[] = [];
  const sectionCounts: Record<string, number> = {};
  const duplicateTracker = new Map<string, number>();
  const blockIndexes: Record<string, number> = {};

  for (const [key, value] of Object.entries(parsed)) {
    if (isArrayTable(value)) {
      sectionCounts[key] = value.length;

      value.forEach((item, index) => {
        const name = typeof item.name === 'string' && item.name.trim()
          ? item.name.trim()
          : `${key} #${index + 1}`;
        const block = sectionBlocks[key]?.[blockIndexes[key] || 0];
        blockIndexes[key] = (blockIndexes[key] || 0) + 1;

        sections.push({
          sectionKey: key,
          index,
          name,
          data: item,
          startLine: block?.startLine || 0,
          endLine: block?.endLine || 0,
          block: block?.content || '',
        });

        const trackerKey = `${key}:${name}`;
        duplicateTracker.set(trackerKey, (duplicateTracker.get(trackerKey) || 0) + 1);
      });

      continue;
    }

    rootEntries.push({
      key,
      value,
      kind: isPlainObject(value) ? 'table' : 'value',
    });
  }

  return {
    rootEntries,
    sections,
    sectionCounts,
    sectionKeys: Object.keys(sectionCounts),
    duplicateNames: Array.from(duplicateTracker.entries())
      .filter(([, count]) => count > 1)
      .map(([compoundKey, count]) => {
        const separatorIndex = compoundKey.indexOf(':');
        return {
          sectionKey: compoundKey.slice(0, separatorIndex),
          name: compoundKey.slice(separatorIndex + 1),
          count,
        };
      }),
  };
};

interface SectionBlock {
  startLine: number;
  endLine: number;
  content: string;
}

const arrayTablePattern = /^\s*\[\[([^\]]+)]]\s*(?:#.*)?$/;
const tablePattern = /^\s*\[([^\]]+)]\s*(?:#.*)?$/;

const collectSectionBlocks = (raw: string): Record<string, SectionBlock[]> => {
  const lines = raw.split('\n');
  const blocks: Record<string, SectionBlock[]> = {};
  let current: { key: string; start: number } | null = null;

  const pushCurrent = (end: number) => {
    if (!current) return;
    const content = lines.slice(current.start, end).join('\n').trimEnd();
    const list = blocks[current.key] || [];
    list.push({
      startLine: current.start,
      endLine: end,
      content,
    });
    blocks[current.key] = list;
  };

  lines.forEach((line, index) => {
    const arrayMatch = line.match(arrayTablePattern);
    if (arrayMatch) {
      pushCurrent(index);
      current = { key: arrayMatch[1].trim(), start: index };
      return;
    }

    if (current && tablePattern.test(line)) {
      pushCurrent(index);
      current = null;
    }
  });

  pushCurrent(lines.length);
  return blocks;
};

const serializeTomlValue = (value: SerializableValue): string => {
  if (Array.isArray(value)) {
    return `[${value.map((item) => serializeTomlValue(item)).join(', ')}]`;
  }

  if (typeof value === 'string') {
    return JSON.stringify(value);
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? String(value) : '0';
  }

  return value ? 'true' : 'false';
};

export const serializeSectionBlock = (
  sectionKey: string,
  entries: Array<[string, SerializableValue]>,
): string => {
  const lines = [`[[${sectionKey}]]`];

  for (const [key, value] of entries) {
    lines.push(`${key} = ${serializeTomlValue(value)}`);
  }

  return `${lines.join('\n')}\n`;
};

export const appendSectionBlock = (raw: string, block: string): string => {
  const trimmed = raw.trimEnd();
  return trimmed ? `${trimmed}\n\n${block.trimEnd()}\n` : `${block.trimEnd()}\n`;
};

export const removeSectionBlock = (
  raw: string,
  section: Pick<ParsedSection, 'startLine' | 'endLine'>,
): string => {
  if (section.startLine < 0 || section.endLine <= section.startLine) {
    throw new Error('无法定位要删除的段落。');
  }
  const lines = raw.split('\n');
  lines.splice(section.startLine, section.endLine - section.startLine);
  return `${lines.join('\n').trimEnd()}\n`;
};
