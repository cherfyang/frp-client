<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { marked } from 'marked';
import frpGuideMarkdown from '../docs/frp-guide.md?raw';
import {
  appendSectionBlock,
  formatValuePreview,
  parseFrpcDocument,
  serializeSectionBlock,
  type ParsedFrpcDocument,
  type ParsedSection,
} from './frpc';
import {
  defaultTemplateIdBySection,
  detectTemplatePresetId,
  sectionGroupMap,
  sectionGroups,
  templatePresetMap,
  type FieldKind,
  type FieldSchema,
  type FieldState,
  type SectionKey,
  type SerializableFieldValue,
} from './frpcSchema';

type ViewId = 'guide' | 'configs' | 'browse' | 'editor' | 'add';
type SourceMode = 'project' | 'upload' | 'stored';

type FrpControlPayload = {
  success?: boolean;
  error?: string;
  code?: string;
};

type FrpControlApiError = Error & {
  code?: string;
};

interface ConfigListItem {
  slot: number;
  fileName: string;
  name: string;
  description: string;
  updatedAt: string;
}

interface ConfigDocumentPayload {
  mode: 'project' | 'stored';
  fileName: string;
  displayName: string;
  configPath: string;
  content: string;
  slot?: number;
  name?: string;
  description?: string;
  updatedAt?: string;
}

interface ConfigListPayload {
  capacity: number;
  items: ConfigListItem[];
}

interface ExtraField {
  id: number;
  key: string;
  kind: Exclude<FieldKind, 'select'>;
  value: string;
}

type PickerWindow = Window & {
  showOpenFilePicker?: (options?: {
    multiple?: boolean;
    excludeAcceptAllOption?: boolean;
    types?: Array<{
      description?: string;
      accept: Record<string, string[]>;
    }>;
  }) => Promise<FileSystemFileHandle[]>;
};

const defaultFileLabel = '当前目录 / frpc.toml';
const frpcBinaryMissingCode = 'frpc_binary_missing';

const moduleTabs: Array<{ id: ViewId; label: string }> = [
  { id: 'guide', label: 'frp说明' },
  { id: 'configs', label: '配置列表' },
  { id: 'browse', label: '查看段落' },
  { id: 'editor', label: '配置文件' },
  { id: 'add', label: '添加段落' },
];

const sourceText = ref('');
const parsedDocument = ref<ParsedFrpcDocument>(parseFrpcDocument(''));
const fileName = ref(defaultFileLabel);
const fileHandle = ref<FileSystemFileHandle | null>(null);
const sourceMode = ref<SourceMode>('project');
const currentStoredSlot = ref<number | null>(null);
const currentStoredName = ref('');
const currentStoredDescription = ref('');
const currentStoredUpdatedAt = ref('');
const parseError = ref('');
const actionError = ref('');
const isRestartingFrp = ref(false);
const restartButtonText = ref('重启frp服务');
const activeView = ref<ViewId>('editor');
const fileInputRef = ref<HTMLInputElement | null>(null);
const configProfiles = ref<ConfigListItem[]>([]);
const configCapacity = ref(20);
const isSaveProfileDialogOpen = ref(false);
const isSavingProfile = ref(false);
const saveProfileName = ref('');
const saveProfileDescription = ref('');
const saveProfileError = ref('');

const selectedSection = ref<SectionKey>('proxies');
const selectedTemplateId = ref(defaultTemplateIdBySection.proxies);
const customSectionName = ref('');
const formValues = reactive<Record<string, FieldState>>({});
const extraFields = ref<ExtraField[]>([]);
const formError = ref('');
const nextExtraFieldId = ref(1);

const currentSectionGroup = computed(() => sectionGroupMap[selectedSection.value]);
const currentPreset = computed(() => templatePresetMap[selectedTemplateId.value]);
const topLevelError = computed(() => parseError.value || actionError.value);
const canWriteBack = computed(() => sourceMode.value !== 'upload' || Boolean(fileHandle.value));
const guideHtml = computed(() => marked.parse(frpGuideMarkdown) as string);
const visibleFields = computed(() =>
  currentPreset.value.fields.filter((field) => !field.showWhen || field.showWhen(formValues)),
);
const presetOptions = computed(() =>
  currentSectionGroup.value.templates.map((template) => ({
    label: template.label,
    value: template.id,
  })),
);
const usedProfileCount = computed(() => configProfiles.value.length);
const editorSourceSummary = computed(() => {
  if (sourceMode.value === 'project') {
    return '当前目录运行配置，可直接保存回 frpc.toml。';
  }

  if (sourceMode.value === 'stored') {
    return `配置列表槽位 ${currentStoredSlot.value ?? '-'}，保存会写回原始槽位文件。`;
  }

  return canWriteBack.value
    ? '本地上传文件，保存会写回浏览器选择的原文件。'
    : '本地上传文件，没有句柄时会下载新的 frpc.toml。';
});
const saveButtonLabel = computed(() => {
  if (sourceMode.value === 'project') {
    return '保存当前目录 frpc.toml';
  }

  if (sourceMode.value === 'stored') {
    return `保存 ${fileName.value}`;
  }

  return canWriteBack.value ? '保存到原文件' : '下载 frpc.toml';
});

const preferredOrder = ['name', 'type', 'serverName', 'localIP', 'localPort', 'remotePort', 'bindAddr', 'bindPort'];

const sortKeys = (left: string, right: string) => {
  const leftIndex = preferredOrder.indexOf(left);
  const rightIndex = preferredOrder.indexOf(right);

  if (leftIndex === -1 && rightIndex === -1) {
    return left.localeCompare(right);
  }

  if (leftIndex === -1) {
    return 1;
  }

  if (rightIndex === -1) {
    return -1;
  }

  return leftIndex - rightIndex;
};

const groupedSections = computed(() => {
  const groups: Array<{ key: string; items: ParsedSection[]; columns: string[] }> = [];
  const bucket = new Map<string, ParsedSection[]>();

  for (const section of parsedDocument.value.sections) {
    const items = bucket.get(section.sectionKey) || [];
    items.push(section);
    bucket.set(section.sectionKey, items);
  }

  for (const key of parsedDocument.value.sectionKeys) {
    const items = bucket.get(key) || [];
    const columns = Array.from(new Set(items.flatMap((item) => Object.keys(item.data)))).sort(sortKeys);
    groups.push({ key, items, columns });
  }

  return groups;
});

const clearStoredProfileMeta = () => {
  currentStoredSlot.value = null;
  currentStoredName.value = '';
  currentStoredDescription.value = '';
  currentStoredUpdatedAt.value = '';
};

const applyStoredProfileMeta = (payload: ConfigDocumentPayload) => {
  currentStoredSlot.value = payload.slot ?? null;
  currentStoredName.value = payload.name || '';
  currentStoredDescription.value = payload.description || '';
  currentStoredUpdatedAt.value = payload.updatedAt || '';
};

const applyParsedSource = (
  nextSource: string,
  options?: {
    fileLabel?: string;
    handle?: FileSystemFileHandle | null;
    sourceMode?: SourceMode;
  },
) => {
  const normalized = `${nextSource.trimEnd()}\n`;
  const nextParsed = parseFrpcDocument(normalized);

  sourceText.value = normalized;
  parsedDocument.value = nextParsed;
  parseError.value = '';
  actionError.value = '';

  if (options?.fileLabel) {
    fileName.value = options.fileLabel;
  }

  if (options?.handle !== undefined) {
    fileHandle.value = options.handle;
  }

  if (options?.sourceMode) {
    sourceMode.value = options.sourceMode;
  }
};

const requestJson = async <T,>(url: string, init?: RequestInit) => {
  const response = await fetch(url, init);
  const contentType = response.headers.get('content-type') || '';
  const rawText = await response.text();
  let payload: FrpControlPayload | T | undefined;

  if (rawText.trim()) {
    try {
      payload = JSON.parse(rawText) as T | FrpControlPayload;
    } catch {
      payload = undefined;
    }
  }

  if (!response.ok) {
    const errorPayload = (payload || {}) as FrpControlPayload;
    const error = new Error(errorPayload.error || '请求失败。') as FrpControlApiError;
    error.code = errorPayload.code;
    throw error;
  }

  if (!payload) {
    const isHtml = contentType.includes('text/html') || rawText.trimStart().startsWith('<!doctype html');
    throw new Error(
      isHtml
        ? '接口返回了页面 HTML，当前 6633 端口很可能还在运行旧的 Vite 开发服务。请先停止旧进程，再重新执行 ./run.sh。'
        : '接口没有返回可解析的 JSON 数据。',
    );
  }

  return payload as T;
};

const applyDocumentPayload = (payload: ConfigDocumentPayload, mode: SourceMode) => {
  applyParsedSource(payload.content, {
    fileLabel: payload.displayName || defaultFileLabel,
    handle: null,
    sourceMode: mode,
  });

  if (mode === 'stored') {
    applyStoredProfileMeta(payload);
    return;
  }

  clearStoredProfileMeta();
};

const loadProjectConfig = async () => {
  try {
    const payload = await requestJson<ConfigDocumentPayload>('/api/config/current');
    applyDocumentPayload(payload, 'project');
  } catch (error) {
    actionError.value = error instanceof Error
      ? `读取当前目录 frpc.toml 失败：${error.message}`
      : '读取当前目录 frpc.toml 失败。';
  }
};

const loadConfigProfiles = async () => {
  try {
    const payload = await requestJson<ConfigListPayload>('/api/config/list');
    configProfiles.value = payload.items;
    configCapacity.value = payload.capacity;
    actionError.value = '';
  } catch (error) {
    actionError.value = error instanceof Error
      ? `读取配置列表失败：${error.message}`
      : '读取配置列表失败。';
  }
};

const loadStoredConfig = async (slot: number) => {
  try {
    const payload = await requestJson<ConfigDocumentPayload>(`/api/config/list/${slot}`);
    applyDocumentPayload(payload, 'stored');
    activeView.value = 'editor';
    actionError.value = '';
  } catch (error) {
    actionError.value = error instanceof Error
      ? `读取配置槽位失败：${error.message}`
      : '读取配置槽位失败。';
  }
};

const callFrpControlApi = async (url: string) => {
  const response = await fetch(url, {
    method: 'POST',
  });

  let payload: FrpControlPayload = {};

  try {
    payload = (await response.json()) as FrpControlPayload;
  } catch {
    payload = {};
  }

  if (!response.ok || !payload.success) {
    const error = new Error(payload.error || 'frp 控制请求失败。') as FrpControlApiError;
    error.code = payload.code;
    throw error;
  }

  return payload;
};

const markRestartSuccess = () => {
  restartButtonText.value = '已重启';
  window.setTimeout(() => {
    restartButtonText.value = '重启frp服务';
  }, 1500);
};

const installFrpcBinary = async () => {
  restartButtonText.value = '安装中...';
  await callFrpControlApi('/api/frp/install');
};

const restartFrpService = async () => {
  if (isRestartingFrp.value) {
    return;
  }

  isRestartingFrp.value = true;
  restartButtonText.value = '重启中...';
  actionError.value = '';

  try {
    await callFrpControlApi('/api/frp/restart');
    markRestartSuccess();
  } catch (error) {
    const nextError = error as FrpControlApiError;
    const message = nextError instanceof Error ? nextError.message : '重启 frp 服务失败。';

    if (
      nextError.code === frpcBinaryMissingCode
      && window.confirm('未找到 frpc 可执行文件,是否现在安装')
    ) {
      try {
        await installFrpcBinary();
        restartButtonText.value = '重启中...';
        await callFrpControlApi('/api/frp/restart');
        markRestartSuccess();
        return;
      } catch (installError) {
        actionError.value = installError instanceof Error ? installError.message : '安装 frpc 失败。';
        restartButtonText.value = '重启frp服务';
        return;
      }
    }

    actionError.value = message;
    restartButtonText.value = '重启frp服务';
  } finally {
    isRestartingFrp.value = false;
  }
};

const resetForm = () => {
  const preset = currentPreset.value;

  for (const key of Object.keys(formValues)) {
    delete formValues[key];
  }

  for (const field of preset.fields) {
    if (field.kind === 'boolean') {
      formValues[field.key] = Boolean(field.defaultValue);
      continue;
    }

    formValues[field.key] = field.defaultValue !== undefined ? String(field.defaultValue) : '';
  }

  if (selectedSection.value !== 'custom') {
    customSectionName.value = '';
  }

  extraFields.value = [];
  formError.value = '';
};

watch(
  selectedSection,
  (nextSection) => {
    const hasPresetInGroup = currentSectionGroup.value.templates.some(
      (template) => template.id === selectedTemplateId.value,
    );

    if (!hasPresetInGroup) {
      selectedTemplateId.value = defaultTemplateIdBySection[nextSection];
      return;
    }

    resetForm();
  },
  { immediate: true, flush: 'sync' },
);

watch(
  selectedTemplateId,
  () => {
    resetForm();
  },
  { flush: 'sync' },
);

const openFallbackFileInput = () => {
  fileInputRef.value?.click();
};

const loadFile = async (file: File, handle: FileSystemFileHandle | null = null) => {
  try {
    const content = await file.text();
    applyParsedSource(content, {
      fileLabel: file.name,
      handle,
      sourceMode: 'upload',
    });
    clearStoredProfileMeta();
    activeView.value = 'editor';
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : '读取文件失败。';
  }
};

const pickTomlFile = async () => {
  const pickerWindow = window as PickerWindow;

  if (!pickerWindow.showOpenFilePicker) {
    openFallbackFileInput();
    return;
  }

  try {
    const [handle] = await pickerWindow.showOpenFilePicker({
      multiple: false,
      excludeAcceptAllOption: true,
      types: [
        {
          description: 'frpc config',
          accept: {
            'application/toml': ['.toml'],
            'text/plain': ['.toml'],
          },
        },
      ],
    });

    if (!handle) {
      return;
    }

    const file = await handle.getFile();
    await loadFile(file, handle);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (message.includes('aborted')) {
      return;
    }

    actionError.value = `选择文件失败：${message}`;
  }
};

const onFallbackFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];

  if (!file) {
    return;
  }

  await loadFile(file);
  input.value = '';
};

const downloadSource = () => {
  const blob = new Blob([sourceText.value], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName.value.endsWith('.toml') ? fileName.value : 'frpc.toml';
  link.click();
  URL.revokeObjectURL(url);
};

const validateSourceBeforeSave = () => {
  try {
    parseFrpcDocument(`${sourceText.value.trimEnd()}\n`);
    parseError.value = '';
    return true;
  } catch (error) {
    parseError.value = error instanceof Error ? error.message : String(error);
    activeView.value = 'editor';
    return false;
  }
};

const saveBackToFile = async () => {
  if (!validateSourceBeforeSave()) {
    return;
  }

  try {
    if (sourceMode.value === 'project') {
      const payload = await requestJson<ConfigDocumentPayload>('/api/config/current/save', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          content: sourceText.value,
        }),
      });

      applyDocumentPayload(payload, 'project');
      return;
    }

    if (sourceMode.value === 'stored' && currentStoredSlot.value) {
      const payload = await requestJson<ConfigDocumentPayload>(`/api/config/list/${currentStoredSlot.value}/save`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          content: sourceText.value,
        }),
      });

      applyDocumentPayload(payload, 'stored');
      await loadConfigProfiles();
      return;
    }

    if (sourceMode.value === 'stored') {
      throw new Error('当前配置槽位缺少编号，无法保存。');
    }

    if (!fileHandle.value) {
      downloadSource();
      applyParsedSource(sourceText.value, {
        fileLabel: fileName.value,
        handle: null,
        sourceMode: 'upload',
      });
      return;
    }

    const writable = await fileHandle.value.createWritable();
    await writable.write(sourceText.value);
    await writable.close();

    applyParsedSource(sourceText.value, {
      fileLabel: fileName.value,
      handle: fileHandle.value,
      sourceMode: 'upload',
    });
  } catch (error) {
    actionError.value = error instanceof Error ? `写回失败：${error.message}` : '写回失败。';
  }
};

const openSaveProfileDialog = () => {
  saveProfileName.value = '';
  saveProfileDescription.value = '';
  saveProfileError.value = '';
  isSaveProfileDialogOpen.value = true;
};

const closeSaveProfileDialog = () => {
  if (isSavingProfile.value) {
    return;
  }

  isSaveProfileDialogOpen.value = false;
  saveProfileError.value = '';
};

const saveToConfigList = async () => {
  if (!validateSourceBeforeSave()) {
    isSaveProfileDialogOpen.value = false;
    return;
  }

  const name = saveProfileName.value.trim();
  if (!name) {
    saveProfileError.value = '命名不能为空。';
    return;
  }

  isSavingProfile.value = true;
  saveProfileError.value = '';

  try {
    await requestJson<ConfigDocumentPayload>('/api/config/list/save', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        name,
        description: saveProfileDescription.value.trim(),
        content: sourceText.value,
      }),
    });

    await loadConfigProfiles();
    actionError.value = '';
    isSaveProfileDialogOpen.value = false;
    activeView.value = 'configs';
  } catch (error) {
    saveProfileError.value = error instanceof Error ? error.message : '保存到配置列表失败。';
  } finally {
    isSavingProfile.value = false;
  }
};

const isEmptyValue = (value: unknown) =>
  value === undefined
  || value === null
  || value === ''
  || (Array.isArray(value) && value.length === 0);

const normalizeFieldValue = (
  field: FieldSchema,
  value: FieldState,
): SerializableFieldValue | undefined => {
  if (field.kind === 'boolean') {
    return value === true ? true : undefined;
  }

  const text = String(value ?? '').trim();
  if (!text) {
    return undefined;
  }

  if (field.kind === 'number') {
    const parsedNumber = Number(text);
    return Number.isFinite(parsedNumber) ? parsedNumber : undefined;
  }

  if (field.kind === 'array') {
    return text
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }

  return text;
};

const inferExtraFieldKind = (value: unknown): ExtraField['kind'] => {
  if (Array.isArray(value)) {
    return 'array';
  }

  if (typeof value === 'boolean') {
    return 'boolean';
  }

  if (typeof value === 'number') {
    return 'number';
  }

  return 'text';
};

const stringifyValueForField = (value: unknown): string => {
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join(', ');
  }

  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }

  return String(value ?? '');
};

const addExtraField = () => {
  extraFields.value.push({
    id: nextExtraFieldId.value,
    key: '',
    kind: 'text',
    value: '',
  });
  nextExtraFieldId.value += 1;
};

const removeExtraField = (fieldId: number) => {
  extraFields.value = extraFields.value.filter((field) => field.id !== fieldId);
};

const hydrateFormFromSection = (section: ParsedSection) => {
  const presetId = detectTemplatePresetId(section.sectionKey, section.data);
  const preset = templatePresetMap[presetId];
  const nextSection = preset?.sectionKey || 'custom';

  selectedSection.value = nextSection;
  selectedTemplateId.value = presetId;
  resetForm();

  if (nextSection === 'custom') {
    customSectionName.value = section.sectionKey;
  }

  const templateFieldKeys = new Set(currentPreset.value.fields.map((field) => field.key));
  const hiddenKeys = new Set(Object.keys(currentPreset.value.hiddenEntries || {}));

  for (const [key, value] of Object.entries(section.data)) {
    if (hiddenKeys.has(key)) {
      continue;
    }

    if (templateFieldKeys.has(key)) {
      const templateField = currentPreset.value.fields.find((field) => field.key === key);
      if (!templateField) {
        continue;
      }

      if (templateField.kind === 'boolean') {
        formValues[key] = Boolean(value);
      } else {
        formValues[key] = stringifyValueForField(value);
      }

      continue;
    }

    extraFields.value.push({
      id: nextExtraFieldId.value,
      key,
      kind: inferExtraFieldKind(value),
      value: stringifyValueForField(value),
    });
    nextExtraFieldId.value += 1;
  }

  activeView.value = 'add';
};

const appendSectionFromForm = () => {
  formError.value = '';

  const sectionName = selectedSection.value === 'custom'
    ? customSectionName.value.trim()
    : currentPreset.value.sectionKey;

  if (!sectionName) {
    formError.value = '请先填写数组表段落名。';
    return;
  }

  const entries: Array<[string, SerializableFieldValue]> = [];

  for (const [key, value] of Object.entries(currentPreset.value.hiddenEntries || {})) {
    entries.push([key, value]);
  }

  for (const field of currentPreset.value.fields) {
    if (field.showWhen && !field.showWhen(formValues)) {
      continue;
    }

    const normalized = normalizeFieldValue(field, formValues[field.key]);

    if (field.required && isEmptyValue(normalized)) {
      formError.value = `字段「${field.label}」不能为空。`;
      return;
    }

    if (field.kind === 'number' && String(formValues[field.key] ?? '').trim() && normalized === undefined) {
      formError.value = `字段「${field.label}」必须是有效数字。`;
      return;
    }

    if (!isEmptyValue(normalized)) {
      entries.push([field.key, normalized as SerializableFieldValue]);
    }
  }

  for (const field of extraFields.value) {
    const key = field.key.trim();
    const value = field.value.trim();

    if (!key && !value) {
      continue;
    }

    if (!key) {
      formError.value = '额外属性缺少 key，请补全后再追加。';
      return;
    }

    if (!value) {
      formError.value = `额外属性「${key}」缺少值。`;
      return;
    }

    if (field.kind === 'number') {
      const parsedNumber = Number(value);
      if (!Number.isFinite(parsedNumber)) {
        formError.value = `额外属性「${key}」必须是有效数字。`;
        return;
      }

      entries.push([key, parsedNumber]);
      continue;
    }

    if (field.kind === 'boolean') {
      entries.push([key, value === 'true']);
      continue;
    }

    if (field.kind === 'array') {
      entries.push([
        key,
        value
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
      ]);
      continue;
    }

    entries.push([key, value]);
  }

  if (!entries.length) {
    formError.value = '至少填写一个字段后才能追加段落。';
    return;
  }

  const block = serializeSectionBlock(sectionName, entries);
  const nextSource = appendSectionBlock(sourceText.value, block);

  try {
    applyParsedSource(nextSource, {
      fileLabel: fileName.value,
    });
    resetForm();
    activeView.value = 'editor';
  } catch (error) {
    formError.value = error instanceof Error ? error.message : String(error);
  }
};

const orderedEntries = (record: Record<string, unknown>) =>
  Object.entries(record).sort(([left], [right]) => sortKeys(left, right));

const formatTableCell = (value: unknown) => {
  if (value === undefined || value === null || value === '') {
    return '-';
  }

  return formatValuePreview(value);
};

const formatTimestamp = (value: string) => {
  if (!value) {
    return '-';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString('zh-CN', {
    hour12: false,
  });
};

onMounted(() => {
  void loadProjectConfig();
  void loadConfigProfiles();
});
</script>

<template>
  <div class="page-shell">
    <header class="module-bar">
      <div class="brand-block">
        <div class="brand-icon" aria-hidden="true">
          <svg viewBox="0 0 80 80" role="presentation">
            <defs>
              <linearGradient id="frp-brand-gradient" x1="0%" x2="100%" y1="0%" y2="100%">
                <stop offset="0%" stop-color="#2563eb" />
                <stop offset="100%" stop-color="#0f766e" />
              </linearGradient>
            </defs>
            <rect x="6" y="6" width="68" height="68" rx="20" fill="url(#frp-brand-gradient)" />
            <path d="M24 24h13a8 8 0 0 1 0 16H24z" fill="#fff" opacity="0.96" />
            <path d="M24 44h20a8 8 0 0 1 0 16H24z" fill="#fff" opacity="0.92" />
            <path d="M37 32h22" stroke="#bfdbfe" stroke-linecap="round" stroke-width="4" />
            <path d="M44 52h15" stroke="#d1fae5" stroke-linecap="round" stroke-width="4" />
            <circle cx="61" cy="32" r="5" fill="#bfdbfe" />
            <circle cx="61" cy="52" r="5" fill="#d1fae5" />
          </svg>
        </div>
        <div class="brand-copy">
          <p>FRP CONTROL HUB</p>
          <strong>frpc 配置中心</strong>
        </div>
      </div>

      <nav class="module-tabs">
        <button
          v-for="tab in moduleTabs"
          :key="tab.id"
          type="button"
          class="tab-btn"
          :class="{ 'tab-btn-active': activeView === tab.id }"
          @click="activeView = tab.id"
        >
          {{ tab.label }}
        </button>
      </nav>

      <button
        class="accent-btn restart-btn"
        type="button"
        :disabled="isRestartingFrp"
        @click="restartFrpService"
      >
        {{ restartButtonText }}
      </button>
    </header>

    <section v-if="topLevelError" class="error-strip">
      {{ topLevelError }}
    </section>

    <article v-if="activeView === 'guide'" class="panel-card">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">frp说明</p>
          <h2>客户端配置说明文档</h2>
        </div>
      </div>

      <section class="markdown-doc" v-html="guideHtml" />
    </article>

    <article v-else-if="activeView === 'configs'" class="panel-card">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">配置列表</p>
          <h2>已保存配置 {{ usedProfileCount }} / {{ configCapacity }}</h2>
        </div>
        <div class="panel-actions">
          <button class="ghost-btn small-btn" type="button" @click="loadConfigProfiles">刷新列表</button>
        </div>
      </div>

      <div v-if="configProfiles.length" class="profile-list">
        <button
          v-for="profile in configProfiles"
          :key="profile.slot"
          type="button"
          class="profile-row"
          @click="loadStoredConfig(profile.slot)"
        >
          <div class="profile-row-top">
            <strong>{{ profile.name }}</strong>
            <span class="profile-file">-- {{ profile.fileName }}</span>
            <span class="count-chip">槽位 {{ profile.slot }}</span>
          </div>
          <p>{{ profile.description || '未填写描述' }}</p>
          <small>更新时间 {{ formatTimestamp(profile.updatedAt) }}</small>
        </button>
      </div>

      <div v-else class="empty-state">
        还没有保存到配置列表的配置。先在“配置文件”里编辑，再点“保存到配置列表”。
      </div>
    </article>

    <section v-else-if="activeView === 'browse'" class="view-stack">
      <article class="panel-card">
        <div class="panel-head">
          <div>
            <p class="panel-kicker">查看段落</p>
            <h2>已有数组表</h2>
          </div>
          <div class="chip-row">
            <span v-for="(count, key) in parsedDocument.sectionCounts" :key="key" class="count-chip">
              {{ key }} × {{ count }}
            </span>
            <span v-if="!parsedDocument.sections.length" class="count-chip muted-chip">
              暂无数组表
            </span>
          </div>
        </div>

        <div v-if="parsedDocument.duplicateNames.length" class="inline-warning">
          <strong>名称重复：</strong>
          <span v-for="item in parsedDocument.duplicateNames" :key="`${item.sectionKey}-${item.name}`">
            {{ item.sectionKey }} / {{ item.name }} × {{ item.count }}
          </span>
        </div>

        <div class="section-group-list">
          <section v-for="group in groupedSections" :key="group.key" class="section-group">
            <div class="group-head">
              <h3>{{ group.key }}</h3>
              <span>{{ group.items.length }} 个</span>
            </div>

            <div class="section-table-wrap">
              <table class="section-table">
                <thead>
                  <tr>
                    <th class="index-column">#</th>
                    <th v-for="column in group.columns" :key="column">
                      {{ column }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in group.items"
                    :key="`${item.sectionKey}-${item.index}`"
                    class="section-row"
                    @click="hydrateFormFromSection(item)"
                  >
                    <td class="index-column">{{ item.index + 1 }}</td>
                    <td v-for="column in group.columns" :key="`${item.index}-${column}`">
                      <code class="table-cell-value">{{ formatTableCell(item.data[column]) }}</code>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>
        </div>
      </article>

      <article class="panel-card">
        <div class="panel-head">
          <div>
            <p class="panel-kicker">根配置</p>
            <h2>基础字段与内嵌表</h2>
          </div>
        </div>

        <div class="root-grid">
          <div v-for="entry in parsedDocument.rootEntries" :key="entry.key" class="root-card">
            <div class="root-card-head">
              <span>{{ entry.kind === 'table' ? `[${entry.key}]` : entry.key }}</span>
            </div>
            <div v-if="entry.kind === 'table'" class="root-table-body">
              <div
                v-for="[childKey, childValue] in orderedEntries(entry.value as Record<string, unknown>)"
                :key="childKey"
                class="root-table-entry"
              >
                <span class="root-table-key">{{ childKey }}</span>
                <code class="root-table-value">{{ formatValuePreview(childValue) }}</code>
              </div>
            </div>
            <code v-else class="root-value">{{ formatValuePreview(entry.value) }}</code>
          </div>
        </div>
      </article>
    </section>

    <article v-else-if="activeView === 'editor'" class="panel-card source-card">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">配置文件</p>
          <h2>{{ fileName }}</h2>
          <p class="editor-meta">{{ editorSourceSummary }}</p>
        </div>
        <div class="panel-actions editor-actions">
          <button class="ghost-btn small-btn" type="button" @click="pickTomlFile">上传文件</button>
          <button class="ghost-btn small-btn" type="button" @click="loadProjectConfig">读取当前目录</button>
          <button class="ghost-btn small-btn" type="button" @click="openSaveProfileDialog">保存到配置列表</button>
          <button class="accent-btn small-btn" type="button" @click="saveBackToFile">
            {{ saveButtonLabel }}
          </button>
        </div>
      </div>

      <textarea
        v-model="sourceText"
        class="source-editor"
        spellcheck="false"
        placeholder="这里会显示当前 frpc.toml 的原始文本"
      />
    </article>

    <article v-else class="panel-card">
      <div class="panel-head">
        <div>
          <p class="panel-kicker">添加段落</p>
          <h2>模板化追加</h2>
        </div>
      </div>

      <div class="form-grid selector-grid">
        <label class="form-field">
          <span>段落分组</span>
          <select v-model="selectedSection" class="field-input">
            <option
              v-for="group in sectionGroups"
              :key="group.key"
              :value="group.key"
            >
              {{ group.label }}
            </option>
          </select>
        </label>

        <label class="form-field">
          <span>官方模板</span>
          <select v-model="selectedTemplateId" class="field-input">
            <option
              v-for="option in presetOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </label>

        <label v-if="selectedSection === 'custom'" class="form-field">
          <span>数组表名</span>
          <input
            v-model="customSectionName"
            class="field-input"
            type="text"
            placeholder="例如 customVisitors"
          />
        </label>
      </div>

      <section class="preset-card">
        <strong>{{ currentPreset.label }}</strong>
        <p>{{ currentPreset.description }}</p>
        <div v-if="currentPreset.hiddenEntries" class="chip-row">
          <span
            v-for="(value, key) in currentPreset.hiddenEntries"
            :key="key"
            class="count-chip"
          >
            {{ key }} = {{ formatValuePreview(value) }}
          </span>
        </div>
      </section>

      <div v-if="visibleFields.length" class="form-grid field-grid">
        <label
          v-for="field in visibleFields"
          :key="field.key"
          class="form-field"
          :class="{ 'boolean-field': field.kind === 'boolean' }"
        >
          <span>
            {{ field.label }}
            <em v-if="field.required">*</em>
          </span>

          <template v-if="field.kind === 'select'">
            <select v-model="formValues[field.key]" class="field-input">
              <option value="">请选择</option>
              <option
                v-for="option in field.options"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </template>

          <template v-else-if="field.kind === 'boolean'">
            <label class="toggle-input">
              <input v-model="formValues[field.key]" type="checkbox" />
              <span>勾选后写入 `true`</span>
            </label>
          </template>

          <template v-else>
            <input
              v-model="formValues[field.key]"
              class="field-input"
              :type="field.kind === 'number' ? 'number' : 'text'"
              :placeholder="field.placeholder"
            />
          </template>

          <small v-if="field.help">{{ field.help }}</small>
        </label>
      </div>

      <div class="extra-field-block">
        <div class="extra-field-head">
          <h3>额外属性</h3>
          <button class="ghost-btn small-btn" type="button" @click="addExtraField">添加属性</button>
        </div>

        <div v-if="extraFields.length" class="extra-field-list">
          <div v-for="field in extraFields" :key="field.id" class="extra-row">
            <input
              v-model="field.key"
              class="field-input"
              type="text"
              placeholder="key"
            />
            <select v-model="field.kind" class="field-input short-input">
              <option value="text">文本</option>
              <option value="number">数字</option>
              <option value="boolean">布尔</option>
              <option value="array">数组</option>
            </select>
            <template v-if="field.kind === 'boolean'">
              <select v-model="field.value" class="field-input short-input">
                <option value="true">true</option>
                <option value="false">false</option>
              </select>
            </template>
            <template v-else>
              <input
                v-model="field.value"
                class="field-input"
                type="text"
                :placeholder="field.kind === 'array' ? 'a, b, c' : 'value'"
              />
            </template>
            <button class="danger-btn" type="button" @click="removeExtraField(field.id)">删除</button>
          </div>
        </div>
      </div>

      <section v-if="formError" class="inline-warning form-warning">
        {{ formError }}
      </section>

      <div class="form-footer">
        <button class="ghost-btn" type="button" @click="resetForm()">重置表单</button>
        <button class="primary-btn" type="button" @click="appendSectionFromForm">
          追加到编辑器
        </button>
      </div>
    </article>

    <div v-if="isSaveProfileDialogOpen" class="modal-backdrop" @click.self="closeSaveProfileDialog">
      <div class="modal-card">
        <div class="panel-head">
          <div>
            <p class="panel-kicker">保存到配置列表</p>
            <h2>填写命名和描述</h2>
          </div>
        </div>

        <div class="form-grid modal-grid">
          <label class="form-field">
            <span>命名 <em>*</em></span>
            <input
              v-model="saveProfileName"
              class="field-input"
              type="text"
              maxlength="40"
              placeholder="例如 初始配置"
            />
          </label>

          <label class="form-field">
            <span>描述</span>
            <textarea
              v-model="saveProfileDescription"
              class="field-input modal-textarea"
              maxlength="120"
              placeholder="例如 最简单的配置"
            />
          </label>
        </div>

        <section v-if="saveProfileError" class="inline-warning form-warning">
          {{ saveProfileError }}
        </section>

        <div class="form-footer">
          <button class="ghost-btn" type="button" :disabled="isSavingProfile" @click="closeSaveProfileDialog">
            取消
          </button>
          <button class="accent-btn" type="button" :disabled="isSavingProfile" @click="saveToConfigList">
            {{ isSavingProfile ? '保存中...' : '保存到配置列表' }}
          </button>
        </div>
      </div>
    </div>

    <input
      ref="fileInputRef"
      class="sr-only"
      type="file"
      accept=".toml,text/plain"
      @change="onFallbackFileChange"
    />
  </div>
</template>
