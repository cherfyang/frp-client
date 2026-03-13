import { constants as fsConstants } from 'node:fs';
import { access, mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

export const SLOT_COUNT = 20;

const DEFAULT_SLOT_CONTENT = (slot) => `# frpc config slot ${slot}\n`;

const normalizeContent = (content) => `${String(content ?? '').trimEnd()}\n`;

const getConfigDirectory = (rootDir) => path.join(rootDir, 'frpc-config');
const getManifestPath = (rootDir) => path.join(getConfigDirectory(rootDir), 'manifest.json');

export const getCurrentConfigPath = (rootDir) => path.join(rootDir, 'frpc.toml');
export const getSlotFileName = (slot) => `frpc-${slot}.toml`;
export const getSlotFilePath = (rootDir, slot) => path.join(getConfigDirectory(rootDir), getSlotFileName(slot));

const validateSlot = (slot) => {
  const nextSlot = Number(slot);
  if (!Number.isInteger(nextSlot) || nextSlot < 1 || nextSlot > SLOT_COUNT) {
    throw new Error(`配置槽位必须在 1-${SLOT_COUNT} 之间。`);
  }

  return nextSlot;
};

const ensureSlotFiles = async (rootDir) => {
  const configDirectory = getConfigDirectory(rootDir);
  await mkdir(configDirectory, { recursive: true });

  for (let slot = 1; slot <= SLOT_COUNT; slot += 1) {
    const slotFilePath = getSlotFilePath(rootDir, slot);

    try {
      await access(slotFilePath, fsConstants.F_OK);
    } catch {
      await writeFile(slotFilePath, DEFAULT_SLOT_CONTENT(slot), 'utf8');
    }
  }
};

const readManifest = async (rootDir) => {
  await ensureSlotFiles(rootDir);
  const manifestPath = getManifestPath(rootDir);

  try {
    const content = await readFile(manifestPath, 'utf8');
    const parsed = JSON.parse(content);

    return Array.isArray(parsed.items) ? parsed.items : [];
  } catch {
    await writeFile(manifestPath, JSON.stringify({ items: [] }, null, 2) + '\n', 'utf8');
    return [];
  }
};

const writeManifest = async (rootDir, items) => {
  const manifestPath = getManifestPath(rootDir);
  const sorted = [...items].sort((left, right) => left.slot - right.slot);
  await writeFile(manifestPath, JSON.stringify({ items: sorted }, null, 2) + '\n', 'utf8');
};

export const ensureConfigWorkspace = async (rootDir) => {
  await ensureSlotFiles(rootDir);
  await readManifest(rootDir);
};

export const readCurrentConfig = async (rootDir) => {
  const configPath = getCurrentConfigPath(rootDir);
  const content = await readFile(configPath, 'utf8');

  return {
    mode: 'project',
    fileName: 'frpc.toml',
    displayName: '当前目录 / frpc.toml',
    configPath,
    content: normalizeContent(content),
  };
};

export const saveCurrentConfig = async (rootDir, content) => {
  const configPath = getCurrentConfigPath(rootDir);
  const normalized = normalizeContent(content);

  await writeFile(configPath, normalized, 'utf8');

  return {
    mode: 'project',
    fileName: 'frpc.toml',
    displayName: '当前目录 / frpc.toml',
    configPath,
    content: normalized,
  };
};

export const listStoredConfigs = async (rootDir) => {
  const items = await readManifest(rootDir);

  return {
    capacity: SLOT_COUNT,
    items: items.map((item) => ({
      slot: item.slot,
      fileName: getSlotFileName(item.slot),
      name: item.name,
      description: item.description || '',
      updatedAt: item.updatedAt,
    })),
  };
};

export const readStoredConfig = async (rootDir, slot) => {
  const nextSlot = validateSlot(slot);
  const items = await readManifest(rootDir);
  const item = items.find((entry) => entry.slot === nextSlot);

  if (!item) {
    throw new Error(`frpc-${nextSlot}.toml 还没有保存内容。`);
  }

  const slotFilePath = getSlotFilePath(rootDir, nextSlot);
  const content = await readFile(slotFilePath, 'utf8');

  return {
    mode: 'stored',
    slot: nextSlot,
    fileName: getSlotFileName(nextSlot),
    displayName: `${item.name} -- ${getSlotFileName(nextSlot)}`,
    name: item.name,
    description: item.description || '',
    updatedAt: item.updatedAt,
    configPath: slotFilePath,
    content: normalizeContent(content),
  };
};

export const saveStoredConfig = async (rootDir, slot, content) => {
  const nextSlot = validateSlot(slot);
  const items = await readManifest(rootDir);
  const index = items.findIndex((entry) => entry.slot === nextSlot);

  if (index === -1) {
    throw new Error(`frpc-${nextSlot}.toml 还没有登记名称，不能直接保存。`);
  }

  const normalized = normalizeContent(content);
  const slotFilePath = getSlotFilePath(rootDir, nextSlot);
  const nextItem = {
    ...items[index],
    updatedAt: new Date().toISOString(),
  };
  const nextItems = [...items];
  nextItems[index] = nextItem;

  await writeFile(slotFilePath, normalized, 'utf8');
  await writeManifest(rootDir, nextItems);

  return {
    mode: 'stored',
    slot: nextSlot,
    fileName: getSlotFileName(nextSlot),
    displayName: `${nextItem.name} -- ${getSlotFileName(nextSlot)}`,
    name: nextItem.name,
    description: nextItem.description || '',
    updatedAt: nextItem.updatedAt,
    configPath: slotFilePath,
    content: normalized,
  };
};

export const saveConfigToList = async (rootDir, payload) => {
  const name = String(payload.name ?? '').trim();
  const description = String(payload.description ?? '').trim();

  if (!name) {
    throw new Error('命名不能为空。');
  }

  const items = await readManifest(rootDir);
  const usedSlots = new Set(items.map((item) => item.slot));
  let slot = null;

  for (let candidate = 1; candidate <= SLOT_COUNT; candidate += 1) {
    if (!usedSlots.has(candidate)) {
      slot = candidate;
      break;
    }
  }

  if (!slot) {
    throw new Error(`配置列表已满，最多保存 ${SLOT_COUNT} 份配置。`);
  }

  const normalized = normalizeContent(payload.content);
  const slotFilePath = getSlotFilePath(rootDir, slot);
  const nextItem = {
    slot,
    name,
    description,
    updatedAt: new Date().toISOString(),
  };

  await writeFile(slotFilePath, normalized, 'utf8');
  await writeManifest(rootDir, [...items, nextItem]);

  return {
    mode: 'stored',
    slot,
    fileName: getSlotFileName(slot),
    displayName: `${name} -- ${getSlotFileName(slot)}`,
    name,
    description,
    updatedAt: nextItem.updatedAt,
    configPath: slotFilePath,
    content: normalized,
  };
};
