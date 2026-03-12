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
  const rootEntries: RootEntry[] = [];
  const sections: ParsedSection[] = [];
  const sectionCounts: Record<string, number> = {};
  const duplicateTracker = new Map<string, number>();

  for (const [key, value] of Object.entries(parsed)) {
    if (isArrayTable(value)) {
      sectionCounts[key] = value.length;

      value.forEach((item, index) => {
        const name = typeof item.name === 'string' && item.name.trim()
          ? item.name.trim()
          : `${key} #${index + 1}`;

        sections.push({
          sectionKey: key,
          index,
          name,
          data: item,
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
