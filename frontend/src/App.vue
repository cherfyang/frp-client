<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import {
  appendSectionBlock,
  formatValuePreview,
  parseFrpcDocument,
  removeSectionBlock,
  serializeSectionBlock,
  type ParsedFrpcDocument,
  type ParsedSection,
} from './frpc';
import {
  defaultTemplateIdBySection,
  sectionGroupMap,
  sectionGroups,
  templatePresetMap,
  type FieldKind,
  type FieldSchema,
  type FieldState,
  type SectionKey,
} from './frpcSchema';

// Wails runtime
import {
  GetSystemInfo,
  GetFrpVersions,
  CheckFrpcInstalled,
  GetFrpcVersion,
  GetSettings,
  SaveSettings,
  ResetSettings,
  ChooseFile,
  ChooseDirectory,
  CheckSettingsFiles,
  GetDownloadTarget,
  GetFrpHelp,
  ListFrpcProcesses,
  KillFrpcProcesses,
  CancelFrpcDownload,
  DownloadFrpc,
  GetDownloadProgress,
  ReadConfig,
  WriteConfig,
  ValidateConfig,
  GetFrpStatus,
  StartFrp,
  StopFrp,
  RestartFrp,
  GetFrpLogs,
  StartDl,
  GetDlProgress,
  CancelDl,
  ListDl,
  RemoveCompletedDl,
} from '../wailsjs/go/main/App';

type ViewId = 'editor' | 'browse' | 'add' | 'help' | 'download' | 'status' | 'logs' | 'settings';
type FrpStatusView = {
  running: boolean;
  pid: number;
  uptime: string;
  version: string;
  logPath: string;
  configPath?: string;
  binaryPath?: string;
};
type AppSettingsView = {
  toolPath: string;
  configPath: string;
  downloadUrl: string;
  theme: 'dark' | 'light';
  autoStart: boolean;
};
type DownloadTargetView = {
  url: string;
  filename: string;
  version: string;
};
type DlTaskView = {
  id: string;
  url: string;
  destPath: string;
  state: string;
  progress: {
    downloaded: number;
    total: number;
    percentage: number;
    speed: number;
    done: boolean;
    error: string;
  };
};
type SettingsFileStatusView = {
  toolExists: boolean;
  configExists: boolean;
  toolPath: string;
  configPath: string;
  toolHelp: string;
  configHelp: string;
  downloadHelp: string;
  manualKillHelp: string;
};
type FrpcProcessInfoView = {
  pids: number[];
  killCommand: string;
  message: string;
};
type LogLineLevel = 'info' | 'warn' | 'error' | 'debug' | 'default';

const moduleTabs: Array<{ id: ViewId; label: string }> = [
  { id: 'editor', label: '配置文件' },
  { id: 'browse', label: '查看段落' },
  { id: 'add', label: '添加段落' },
  { id: 'download', label: '下载管理' },
  { id: 'status', label: '状态' },
  { id: 'logs', label: '日志' },
  { id: 'help', label: '说明' },
  { id: 'settings', label: '设置' },
];

const sourceText = ref('');
const parsedDocument = ref<ParsedFrpcDocument>(parseFrpcDocument(''));
const parseError = ref('');
const actionError = ref('');
const actionSuccess = ref('');
const activeView = ref<ViewId>('editor');
const sourceEditor = ref<HTMLTextAreaElement | null>(null);
const toolPath = ref('');
const configPath = ref('');
const theme = ref<'dark' | 'light'>('dark');
const systemInfo = ref<{ os: string; arch: string } | null>(null);
const settingsDraft = reactive<AppSettingsView>({
  toolPath: '',
  configPath: '',
  downloadUrl: '',
  theme: 'dark',
  autoStart: false,
});
const settingsFileStatus = ref<SettingsFileStatusView | null>(null);
const settingsError = ref('');

const isRestarting = ref(false);
const restartButtonText = ref('重启frp服务');

const frpStatus = ref<FrpStatusView>({ running: false, pid: 0, uptime: '', version: '', logPath: '', configPath: '', binaryPath: '' });
const frpLogs = ref('');
const frpcInstalled = ref(false);
const frpcVersion = ref('');

const versions = ref<string[]>([]);
const selectedVersion = ref('v0.68.0');
const downloadTarget = ref<DownloadTargetView>({ url: '', filename: '', version: '' });
const helpMarkdown = ref('');
const downloadProgress = ref(0);
const isDownloading = ref(false);
const isCancelingDownload = ref(false);

const showDownloadPanel = ref(false);

const dlUrl = ref('');
const dlDir = ref('');
const dlFilename = ref('');
const dlConnections = ref(4);
const dlTasks = ref<DlTaskView[]>([]);
const dlFilter = ref<'active' | 'done' | 'canceled' | 'error'>('active');

const filteredDlTasks = computed(() =>
  dlTasks.value.filter(t => {
    switch (dlFilter.value) {
      case 'active': return t.state === 'downloading' || t.state === 'pending';
      case 'done': return t.state === 'done';
      case 'canceled': return t.state === 'canceled';
      case 'error': return t.state === 'error';
    }
  }),
);

const selectedSection = ref<SectionKey>('proxies');
const selectedTemplateId = ref(defaultTemplateIdBySection.proxies);
const customSectionName = ref('');
const formValues = reactive<Record<string, FieldState>>({});
const extraFields = ref<Array<{ id: number; key: string; kind: Exclude<FieldKind, 'select'>; value: string }>>([]);
const formError = ref('');
const nextExtraFieldId = ref(1);

const currentSectionGroup = computed(() => sectionGroupMap[selectedSection.value]);
const currentPreset = computed(() => templatePresetMap[selectedTemplateId.value]);

const visibleFields = computed(() =>
  currentPreset.value.fields.filter((field) => !field.showWhen || field.showWhen(formValues)),
);

const basicFields = computed(() => visibleFields.value.filter((field) => field.group !== 'advanced'));
const advancedFields = computed(() => visibleFields.value.filter((field) => field.group === 'advanced'));

const presetOptions = computed(() =>
  currentSectionGroup.value.templates.map((template) => ({
    label: template.label,
    value: template.id,
  })),
);

const renderedHelpBlocks = computed(() => renderMarkdown(helpMarkdown.value));
const renderedLogLines = computed(() => {
  const text = frpLogs.value.trim();
  if (!text) return [];
  return text.split('\n').map((content, index) => ({
    id: index,
    content,
    level: detectLogLevel(content),
  })).reverse();
});

const detectLogLevel = (line: string): LogLineLevel => {
  const normalized = line.toLowerCase();
  if (/\b(error|fatal|panic|failed|fail)\b/.test(normalized) || /\[e\]/.test(normalized)) return 'error';
  if (/\b(warn|warning)\b/.test(normalized) || /\[w\]/.test(normalized)) return 'warn';
  if (/\b(info)\b/.test(normalized) || /\[i\]/.test(normalized)) return 'info';
  if (/\b(debug|trace)\b/.test(normalized) || /\[d\]|\[t\]/.test(normalized)) return 'debug';
  return 'default';
};

type HelpBlock =
  | { type: 'heading'; level: number; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'list'; items: string[] }
  | { type: 'code'; lang: string; text: string };

const renderMarkdown = (markdown: string): HelpBlock[] => {
  const blocks: HelpBlock[] = [];
  const lines = markdown.split('\n');
  let paragraph: string[] = [];
  let listItems: string[] = [];
  let codeLang = '';
  let codeLines: string[] | null = null;

  const flushParagraph = () => {
    if (!paragraph.length) return;
    blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
    paragraph = [];
  };
  const flushList = () => {
    if (!listItems.length) return;
    blocks.push({ type: 'list', items: [...listItems] });
    listItems = [];
  };

  for (const line of lines) {
    const fence = line.match(/^```(\w+)?/);
    if (fence) {
      if (codeLines) {
        blocks.push({ type: 'code', lang: codeLang, text: codeLines.join('\n') });
        codeLines = null;
        codeLang = '';
      } else {
        flushParagraph();
        flushList();
        codeLines = [];
        codeLang = fence[1] || '';
      }
      continue;
    }
    if (codeLines) {
      codeLines.push(line);
      continue;
    }
    const trimmed = line.trim();
    if (!trimmed) {
      flushParagraph();
      flushList();
      continue;
    }
    const heading = trimmed.match(/^(#{1,3})\s+(.+)$/);
    if (heading) {
      flushParagraph();
      flushList();
      blocks.push({ type: 'heading', level: heading[1].length, text: stripInlineMarkdown(heading[2]) });
      continue;
    }
    const list = trimmed.match(/^-\s+(.+)$/);
    if (list) {
      flushParagraph();
      listItems.push(stripInlineMarkdown(list[1]));
      continue;
    }
    flushList();
    paragraph.push(stripInlineMarkdown(trimmed));
  }
  flushParagraph();
  flushList();
  return blocks;
};

const stripInlineMarkdown = (text: string) =>
  text.replace(/`([^`]+)`/g, '$1').replace(/\*\*([^*]+)\*\*/g, '$1');

const isRunning = computed(() => frpStatus.value.running);
const proxyCount = computed(() => parsedDocument.value.sectionCounts.proxies || 0);
const visitorCount = computed(() => parsedDocument.value.sectionCounts.visitors || 0);

const rootEntryMap = computed(() => {
  const entries = new Map<string, unknown>();
  for (const entry of parsedDocument.value.rootEntries) {
    entries.set(entry.key, entry.value);
  }
  return entries;
});

const serverSummary = computed(() => {
  const addr = rootEntryMap.value.get('serverAddr');
  const port = rootEntryMap.value.get('serverPort');
  if (!addr && !port) return '未配置服务器';
  return `${addr || '?'}:${port || '?'}`;
});

const authSummary = computed(() => {
  const auth = rootEntryMap.value.get('auth');
  if (!auth || typeof auth !== 'object' || Array.isArray(auth)) return '未配置认证';
  const token = (auth as Record<string, unknown>).token;
  if (typeof token !== 'string' || !token) return '未配置 token';
  return `token ${token.slice(0, 3)}***${token.slice(-3)}`;
});

const duplicateWarnings = computed(() =>
  parsedDocument.value.duplicateNames.map((item) => `${item.sectionKey}/${item.name} 重复 ${item.count} 次`),
);

const applySettingsToState = (settings: AppSettingsView) => {
  toolPath.value = settings.toolPath;
  configPath.value = settings.configPath;
  settingsDraft.downloadUrl = settings.downloadUrl;
  theme.value = settings.theme === 'light' ? 'light' : 'dark';
  settingsDraft.toolPath = settings.toolPath;
  settingsDraft.configPath = settings.configPath;
  settingsDraft.theme = theme.value;
  settingsDraft.autoStart = Boolean(settings.autoStart);
};

const preferredOrder = ['name', 'type', 'serverName', 'localIP', 'localPort', 'remotePort', 'bindAddr', 'bindPort'];

const sortKeys = (left: string, right: string) => {
  const leftIndex = preferredOrder.indexOf(left);
  const rightIndex = preferredOrder.indexOf(right);
  if (leftIndex === -1 && rightIndex === -1) return left.localeCompare(right);
  if (leftIndex === -1) return 1;
  if (rightIndex === -1) return -1;
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

const applyParsedSource = (nextSource: string) => {
  const normalized = `${nextSource.trimEnd()}\n`;
  const nextParsed = parseFrpcDocument(normalized);
  sourceText.value = normalized;
  parsedDocument.value = nextParsed;
  parseError.value = '';
  actionError.value = '';
};

// ========== 初始化 ==========

onMounted(async () => {
  await refreshAll();
});

const refreshAll = async () => {
  await loadSettings();
  await refreshSettingsFileStatus();
  await loadConfig();
  await refreshFrpStatus();
  await checkInstall();
  await loadVersions();
  await refreshDownloadTarget();
  await loadHelp();
  try { systemInfo.value = await GetSystemInfo() as any; } catch { /* ignore */ }
};

const loadSettings = async () => {
  try {
    const settings = await GetSettings() as AppSettingsView;
    applySettingsToState(settings);
  } catch {
    toolPath.value = '';
    configPath.value = '';
  }
};

const refreshSettingsFileStatus = async () => {
  try {
    settingsFileStatus.value = await CheckSettingsFiles() as SettingsFileStatusView;
  } catch {
    settingsFileStatus.value = null;
  }
};

const checkInstall = async () => {
  try {
    frpcInstalled.value = await CheckFrpcInstalled();
    frpcVersion.value = (await GetFrpcVersion()) || '';
  } catch {
    frpcInstalled.value = false;
  }
};

const loadConfig = async () => {
  try {
    const content = await ReadConfig();
    applyParsedSource(content);
  } catch (e: any) {
    actionError.value = `读取配置失败: ${e.message || e}`;
  }
};

const loadVersions = async () => {
  try {
    versions.value = await GetFrpVersions();
    if (versions.value.length) {
      selectedVersion.value = versions.value[0];
    }
    await refreshDownloadTarget();
  } catch { /* ignore */ }
};

const refreshDownloadTarget = async () => {
  try {
    const info = await GetSystemInfo();
    downloadTarget.value = await GetDownloadTarget(selectedVersion.value, info.os, info.arch) as DownloadTargetView;
  } catch {
    downloadTarget.value = { url: '', filename: '', version: selectedVersion.value };
  }
};

const loadHelp = async () => {
  try {
    const info = await GetSystemInfo();
    helpMarkdown.value = await GetFrpHelp(selectedVersion.value, info.os, info.arch);
  } catch {
    helpMarkdown.value = '';
  }
};



const pickToolPath = async () => {
  try {
    const picked = await ChooseFile('选择 frpc 可执行文件');
    if (picked) settingsDraft.toolPath = picked;
  } catch (e: any) {
    settingsError.value = e.message || String(e);
  }
};

const pickConfigPath = async () => {
  try {
    const picked = await ChooseFile('选择 frpc.toml 配置文件');
    if (picked) settingsDraft.configPath = picked;
  } catch (e: any) {
    settingsError.value = e.message || String(e);
  }
};

const saveAppSettings = async () => {
  settingsError.value = '';
  actionError.value = '';
  try {
    const next = await SaveSettings({
      toolPath: settingsDraft.toolPath,
      configPath: settingsDraft.configPath,
      downloadUrl: settingsDraft.downloadUrl,
      theme: settingsDraft.theme,
      autoStart: settingsDraft.autoStart,
    }) as AppSettingsView;
    applySettingsToState(next);
    activeView.value = 'editor';
    actionSuccess.value = '设置已保存';
    await refreshSettingsFileStatus();
    await loadConfig();
    await refreshFrpStatus();
    await checkInstall();
    await refreshDownloadTarget();
    await loadHelp();
  } catch (e: any) {
    settingsError.value = e.message || String(e);
  }
};

const resetAppSettings = async () => {
  settingsError.value = '';
  if (!window.confirm('确认重置为默认设置？这会关闭 frpc 开机自启动设置，但不会删除已有文件。')) return;
  try {
    const next = await ResetSettings() as AppSettingsView;
    applySettingsToState(next);
    actionSuccess.value = '已重置为默认设置';
    await refreshSettingsFileStatus();
    await loadConfig();
    await refreshFrpStatus();
    await checkInstall();
    await refreshDownloadTarget();
    await loadHelp();
  } catch (e: any) {
    settingsError.value = e.message || String(e);
  }
};

// ========== 配置保存 ==========

const validateSourceBeforeSave = () => {
  try {
    parseFrpcDocument(`${sourceText.value.trimEnd()}\n`);
    parseError.value = '';
    return true;
  } catch (error: any) {
    parseError.value = error.message || String(error);
    activeView.value = 'editor';
    return false;
  }
};

const saveConfig = async () => {
  if (!validateSourceBeforeSave()) return;
  actionError.value = '';
  actionSuccess.value = '';
  try {
    await WriteConfig(sourceText.value);
    await refreshSettingsFileStatus();
    actionSuccess.value = '配置已保存';
    setTimeout(() => { actionSuccess.value = ''; }, 2000);
  } catch (e: any) {
    actionError.value = `保存失败: ${e.message || e}`;
  }
};

const validateConfig = async () => {
  actionError.value = '';
  actionSuccess.value = '';
  try {
    const msg = await ValidateConfig();
    actionSuccess.value = msg || '配置验证通过';
  } catch (e: any) {
    actionError.value = e || '配置验证失败';
  }
};

// ========== 下载管理 ==========

const startDownload = async () => {
  if (isDownloading.value) return;
  isDownloading.value = true;
  isCancelingDownload.value = false;
  downloadProgress.value = 0;
  actionError.value = '';
  actionSuccess.value = '';

  const info = await GetSystemInfo();
  try {
    await DownloadFrpc(selectedVersion.value, '', info.os, info.arch);
    await checkInstall();
    await refreshSettingsFileStatus();
    actionSuccess.value = 'frpc 下载安装完成';
    isDownloading.value = false;
    isCancelingDownload.value = false;
  } catch (e: any) {
    const message = e.message || String(e);
    actionError.value = message.includes('下载已停止') ? '下载已停止' : `下载失败: ${message}`;
    isDownloading.value = false;
    isCancelingDownload.value = false;
  }
};

const stopDownload = async () => {
  if (!isDownloading.value || isCancelingDownload.value) return;
  isCancelingDownload.value = true;
  try {
    await CancelFrpcDownload();
  } catch { /* ignore */ }
};

let progressTimer: ReturnType<typeof setInterval> | null = null;

watch(isDownloading, (val) => {
  if (val) {
    progressTimer = setInterval(async () => {
      try {
        downloadProgress.value = await GetDownloadProgress();
      } catch { /* ignore */ }
    }, 500);
  } else {
    if (progressTimer) {
      clearInterval(progressTimer);
      progressTimer = null;
    }
  }
});

watch(selectedVersion, () => {
  refreshDownloadTarget();
  loadHelp();
});

// ========== 进程管理 ==========

const refreshFrpStatus = async () => {
  try {
    frpStatus.value = await GetFrpStatus();
  } catch { /* ignore */ }
};

const refreshLogs = async () => {
  try {
    frpLogs.value = await GetFrpLogs(50);
  } catch { /* ignore */ }
};

const confirmAndKillExistingFrpc = async () => {
  const info = await ListFrpcProcesses() as FrpcProcessInfoView;
  if (!info.pids?.length) return true;
  const message = `${info.message}\n\n${info.killCommand ? `手动命令: ${info.killCommand}` : ''}`;
  if (!window.confirm(message)) return false;
  try {
    await KillFrpcProcesses(info.pids);
    return true;
  } catch (e: any) {
    const suffix = info.killCommand ? `\n请手动执行: ${info.killCommand}` : '';
    actionError.value = `${e.message || String(e)}${suffix}`;
    return false;
  }
};

let statusTimer: ReturnType<typeof setInterval> | null = null;

onMounted(() => {
  statusTimer = setInterval(refreshFrpStatus, 3000);
});

onBeforeUnmount(() => {
  if (statusTimer) clearInterval(statusTimer);
  if (progressTimer) clearInterval(progressTimer);
  if (dlTimer) clearInterval(dlTimer);
});

const handleStart = async () => {
  actionError.value = '';
  await refreshLogs();
  try {
    const canStart = await confirmAndKillExistingFrpc();
    if (!canStart) return;
    await StartFrp();
    actionSuccess.value = 'frpc 已启动';
    await refreshFrpStatus();
    await refreshLogs();
  } catch (e: any) {
    actionError.value = e.message || String(e);
    await refreshLogs();
  }
};

const handleStop = async () => {
  actionError.value = '';
  try {
    await StopFrp();
    actionSuccess.value = 'frpc 已停止';
    await refreshFrpStatus();
  } catch (e: any) {
    actionError.value = e.message || String(e);
  }
};

const handleRestart = async () => {
  if (isRestarting.value) return;
  isRestarting.value = true;
  restartButtonText.value = '重启中...';
  actionError.value = '';
  try {
    if (!frpcInstalled.value) {
      const ok = window.confirm('未找到 frpc, 是否现在安装?');
      if (ok) {
        await startDownload();
        await refreshFrpStatus();
      }
      restartButtonText.value = '重启frp服务';
      return;
    }
    const canRestart = await confirmAndKillExistingFrpc();
    if (!canRestart) {
      restartButtonText.value = '重启frp服务';
      return;
    }
    await RestartFrp();
    restartButtonText.value = '已重启';
    setTimeout(() => { restartButtonText.value = '重启frp服务'; }, 1500);
    await refreshFrpStatus();
  } catch (e: any) {
    actionError.value = e.message || String(e);
    restartButtonText.value = '重启frp服务';
  } finally {
    isRestarting.value = false;
  }
};



const jumpToSectionSource = async (section: ParsedSection) => {
  activeView.value = 'editor';
  await nextTick();
  const editor = sourceEditor.value;
  if (!editor || section.startLine < 0) return;
  const lines = sourceText.value.split('\n');
  const start = lines.slice(0, section.startLine).join('\n').length + (section.startLine > 0 ? 1 : 0);
  const end = start + (section.block || '').length;
  editor.focus();
  editor.setSelectionRange(start, end);
  editor.scrollTop = Math.max(0, section.startLine * 22 - editor.clientHeight / 3);
};

const duplicateSection = (section: ParsedSection) => {
  if (!section.block) {
    actionError.value = '无法定位要复制的段落';
    return;
  }
  let block = section.block;
  const currentName = typeof section.data.name === 'string' ? section.data.name : '';
  if (currentName) {
    const nextName = `${currentName}_copy`;
    block = block.replace(/^(\s*name\s*=\s*)(.+)$/m, `$1${JSON.stringify(nextName)}`);
  }
  try {
    applyParsedSource(appendSectionBlock(sourceText.value, block));
    actionSuccess.value = '段落已复制到配置末尾';
    activeView.value = 'browse';
  } catch (e: any) {
    actionError.value = e.message || String(e);
  }
};

const deleteSection = (section: ParsedSection) => {
  if (!window.confirm(`确认删除段落 "${section.name}"?`)) return;
  try {
    applyParsedSource(removeSectionBlock(sourceText.value, section));
    actionSuccess.value = '段落已删除，保存后生效';
  } catch (e: any) {
    actionError.value = e.message || String(e);
  }
};

// ========== 模板表单 ==========

const resetForm = () => {
  const preset = currentPreset.value;
  for (const key of Object.keys(formValues)) delete formValues[key];
  for (const field of preset.fields) {
    if (field.kind === 'boolean') {
      formValues[field.key] = Boolean(field.defaultValue);
    } else {
      formValues[field.key] = field.defaultValue !== undefined ? String(field.defaultValue) : '';
    }
  }
  if (selectedSection.value !== 'custom') customSectionName.value = '';
  extraFields.value = [];
  formError.value = '';
};

watch(selectedSection, (nextSection) => {
  const has = currentSectionGroup.value.templates.some((t) => t.id === selectedTemplateId.value);
  if (!has) selectedTemplateId.value = defaultTemplateIdBySection[nextSection];
  resetForm();
}, { immediate: true, flush: 'sync' });

watch(selectedTemplateId, () => resetForm(), { flush: 'sync' });

const addExtraField = () => {
  extraFields.value.push({ id: nextExtraFieldId.value++, key: '', kind: 'text', value: '' });
};

const removeExtraField = (id: number) => {
  extraFields.value = extraFields.value.filter((f) => f.id !== id);
};

const appendSectionFromForm = () => {
  formError.value = '';
  actionError.value = '';
  const fieldPairs: Array<[FieldSchema, FieldState]> = [];
  for (const field of currentPreset.value.fields) {
    if (field.showWhen && !field.showWhen(formValues)) continue;
    let val = formValues[field.key];
    if (field.required && (val === undefined || val === '')) {
      formError.value = `"${field.label}" 是必填项。`;
      return;
    }
    fieldPairs.push([field, val !== undefined ? val : '']);
  }

  const extraPairs = extraFields.value
    .filter((ef) => ef.key.trim())
    .map((ef) => ({ key: ef.key.trim(), kind: ef.kind, value: ef.value }));

  try {
    const entries: Array<[string, any]> = Object.entries(currentPreset.value.hiddenEntries || {});
    for (const [field, val] of fieldPairs) {
      if (val === '' && !field.required) continue;
      if (field.kind === 'number') {
        const numberValue = Number(val);
        if (!Number.isFinite(numberValue)) {
          throw new Error(`"${field.label}" 必须是数字。`);
        }
        validatePortField(field.key, numberValue);
        entries.push([field.key, numberValue]);
        continue;
      }
      if (field.kind === 'boolean') {
        entries.push([field.key, Boolean(val)]);
        continue;
      }
      if (field.kind === 'array') {
        entries.push([field.key, String(val).split(',').map(s => s.trim()).filter(Boolean)]);
        continue;
      }
      entries.push([field.key, String(val)]);
    }
    for (const ep of extraPairs) {
      entries.push([ep.key, coerceExtraFieldValue(ep.kind, ep.value)]);
    }

    const block = serializeSectionBlock(
      selectedSection.value === 'custom' && customSectionName.value ? customSectionName.value : selectedSection.value,
      entries,
    );
    sourceText.value = appendSectionBlock(sourceText.value, block);
    applyParsedSource(sourceText.value);
    activeView.value = 'editor';
    actionSuccess.value = '段落已追加，保存后生效';
  } catch (e: any) {
    formError.value = e.message || String(e);
  }
};

const validatePortField = (key: string, value: number) => {
  if (!key.toLowerCase().includes('port')) return;
  const min = key === 'remotePort' ? 0 : 1;
  if (!Number.isInteger(value) || value < min || value > 65535) {
    throw new Error(`${key} 必须是 ${min}-${65535} 之间的整数。`);
  }
};

const coerceExtraFieldValue = (kind: Exclude<FieldKind, 'select'>, value: string) => {
  if (kind === 'number') {
    const numberValue = Number(value);
    if (!Number.isFinite(numberValue)) throw new Error('额外属性的数字值无效。');
    return numberValue;
  }
  if (kind === 'boolean') return value === 'true';
  if (kind === 'array') return value.split(',').map(s => s.trim()).filter(Boolean);
  return value;
};

// ========== 下载面板 ==========

let dlTimer: ReturnType<typeof setInterval> | null = null;

const refreshDlTasks = async () => {
  try {
    dlTasks.value = await ListDl() as DlTaskView[];
    const hasActive = dlTasks.value.some(t => t.state === 'downloading' || t.state === 'pending');
    if (!hasActive && dlTimer) {
      clearInterval(dlTimer);
      dlTimer = null;
    }
  } catch { /* ignore */ }
};

const extractFilename = (urlStr: string): string => {
  try {
    const u = new URL(urlStr);
    const parts = u.pathname.split('/').filter(Boolean);
    if (parts.length > 0) {
      const last = parts[parts.length - 1];
      if (last && !last.endsWith('/')) return decodeURIComponent(last);
    }
    for (const key of ['path', 'file', 'filename']) {
      const val = u.searchParams.get(key);
      if (val) {
        const segs = val.split('/').filter(Boolean);
        if (segs.length > 0) return decodeURIComponent(segs[segs.length - 1]);
      }
    }
  } catch {}
  const qIdx = urlStr.indexOf('?');
  const base = qIdx >= 0 ? urlStr.substring(0, qIdx) : urlStr;
  const segments = base.split('/').filter(Boolean);
  return segments.length > 0 ? segments[segments.length - 1] : 'download';
};

const pickDir = async () => {
  try {
    const picked = await ChooseDirectory('选择保存目录');
    if (picked) dlDir.value = picked;
  } catch { /* ignore */ }
};

const startDl = async () => {
  if (!dlUrl.value || !dlDir.value || !dlFilename.value) return;
  actionError.value = '';
  actionSuccess.value = '';
  const destPath = dlDir.value.replace(/\/$/, '') + '/' + dlFilename.value;
  try {
    await StartDl(dlUrl.value, destPath, dlConnections.value);
    if (!dlTimer) {
      await refreshDlTasks();
      dlTimer = setInterval(refreshDlTasks, 500);
    }
    dlUrl.value = '';
    dlDir.value = '';
    dlFilename.value = '';
  } catch (e: any) {
    actionError.value = e?.message || String(e);
  }
};

const cancelDl = async (taskId: string) => {
  try {
    await CancelDl(taskId);
  } catch { /* ignore */ }
};

const removeCompleted = async () => {
  try {
    await RemoveCompletedDl();
    await refreshDlTasks();
  } catch { /* ignore */ }
};

const dlFilterCounts = computed(() => ({
  active: dlTasks.value.filter(t => t.state === 'downloading' || t.state === 'pending').length,
  done: dlTasks.value.filter(t => t.state === 'done').length,
  canceled: dlTasks.value.filter(t => t.state === 'canceled').length,
  error: dlTasks.value.filter(t => t.state === 'error').length,
}));

const formatBytes = (bytes: number): string => {
  if (bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
};

const formatEta = (downloaded: number, total: number, speed: number): string => {
  if (!speed || total <= 0 || downloaded >= total) return '--';
  const eta = Math.ceil((total - downloaded) / speed);
  if (eta < 60) return `${eta}秒`;
  if (eta < 3600) return `${Math.floor(eta / 60)}分${eta % 60}秒`;
  return `${Math.floor(eta / 3600)}时${Math.floor((eta % 3600) / 60)}分`;
};

watch(dlUrl, (url) => {
  if (url) dlFilename.value = extractFilename(url);
});
</script>

<template>
  <div class="app-container" :class="`theme-${theme}`">
    <header class="app-header">
      <div class="header-left">
        <h1>FRP Client</h1>
        <span class="status-dot" :class="{ on: isRunning }"></span>
        <span class="status-text">{{ isRunning ? `运行中 · PID ${frpStatus.pid}` : '已停止' }}</span>
        <span v-if="isRunning && frpStatus.uptime" class="uptime">{{ frpStatus.uptime }}</span>
      </div>
      <div class="header-right">
        <button class="btn btn-sm btn-ghost" @click="showDownloadPanel = !showDownloadPanel">
          {{ showDownloadPanel ? '收起安装' : frpcInstalled ? `v${frpcVersion || '?'}` : '安装 frpc' }}
        </button>
        <button v-if="frpcInstalled && !isRunning" class="btn btn-sm btn-green" @click="handleStart">启动</button>
        <button v-if="isRunning" class="btn btn-sm btn-red" @click="handleStop">停止</button>
        <button v-if="frpcInstalled" class="btn btn-sm btn-blue" :disabled="isRestarting" @click="handleRestart">{{ restartButtonText }}</button>
      </div>
    </header>

    <!-- 下载面板 -->
    <section v-if="showDownloadPanel" class="panel">
      <div class="download-head">
        <div>
          <span class="label">当前下载链接</span>
          <code>{{ downloadTarget.url || '读取中...' }}</code>
        </div>
        <span class="download-file">{{ downloadTarget.filename }}</span>
      </div>
      <div class="panel-row">
        <div class="panel-cell">
          <label class="label">版本</label>
          <select v-model="selectedVersion" class="input">
            <option v-for="v in versions" :key="v" :value="v">{{ v }}</option>
          </select>
        </div>
        <div class="panel-cell">
          <label class="label">下载命令</label>
          <input class="input command-input" type="text" readonly :value="`curl -L -o ${downloadTarget.filename || 'frp.tar.gz'} ${downloadTarget.url || ''}`" />
        </div>
        <div class="panel-cell panel-btn">
          <button class="btn btn-blue" :disabled="isDownloading" @click="startDownload">
            {{ frpcInstalled ? '更新 frpc' : '下载安装' }}
          </button>
          <button v-if="isDownloading" class="btn btn-red" :disabled="isCancelingDownload" @click="stopDownload">
            {{ isCancelingDownload ? '停止中...' : '停止下载' }}
          </button>
        </div>
      </div>
      <div v-if="isDownloading" class="progress">
        <div class="progress-fill" :style="{ width: downloadProgress + '%' }"></div>
      </div>
    </section>

    <!-- Tab 导航 -->
    <nav class="tabs">
      <button
        v-for="tab in moduleTabs"
        :key="tab.id"
        class="tab"
        :class="{ active: activeView === tab.id }"
        @click="activeView = tab.id"
      >{{ tab.label }}</button>
    </nav>

    <!-- ====== 配置文件 (Editor) ====== -->
    <article v-if="activeView === 'editor'" class="body">
      <div class="editor-bar">
        <div class="editor-bar-left">
          <button class="btn btn-blue" @click="saveConfig">保存配置</button>
          <button class="btn btn-ghost" @click="loadConfig">重新读取</button>
          <button class="btn btn-ghost" @click="validateConfig" :disabled="!frpcInstalled">验证配置</button>
        </div>
        <span class="editor-path">{{ configPath || 'frpc.toml' }}</span>
      </div>
      <div v-if="parseError" class="msg msg-err msg-sm">{{ parseError }}</div>
      <textarea
        ref="sourceEditor"
        class="editor-area"
        v-model="sourceText"
        spellcheck="false"
        placeholder="在此编辑 frpc.toml ..."
      ></textarea>
    </article>

    <!-- ====== 查看段落 (Browse) ====== -->
    <article v-if="activeView === 'browse'" class="body">
      <div v-if="!groupedSections.length" class="empty">暂无段落数据</div>
      <div v-for="group in groupedSections" :key="group.key" class="block">
        <div class="block-head">
          <div>
            <div class="block-title">{{ group.key }}</div>
            <div class="block-subtitle">{{ group.items.length }} 个段落</div>
          </div>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th v-for="col in group.columns" :key="col">{{ col }}</th>
                <th class="actions-col">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, idx) in group.items" :key="idx">
                <td v-for="col in group.columns" :key="col">{{ formatValuePreview(item.data[col]) }}</td>
                <td class="row-actions">
                  <button class="btn btn-xs btn-ghost" type="button" @click="jumpToSectionSource(item)">定位</button>
                  <button class="btn btn-xs btn-ghost" type="button" @click="duplicateSection(item)">复制</button>
                  <button class="btn btn-xs btn-red" type="button" @click="deleteSection(item)">删除</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </article>

    <!-- ====== 添加段落 (Add) ====== -->
    <article v-if="activeView === 'add'" class="body">
      <div class="add-workspace">
        <aside class="template-rail">
          <div class="rail-group">
            <span class="rail-label">段落类型</span>
            <button
              v-for="g in sectionGroups"
              :key="g.key"
              type="button"
              class="template-option"
              :class="{ active: selectedSection === g.key }"
              @click="selectedSection = g.key"
            >
              <strong>{{ g.label }}</strong>
              <small>{{ g.key === 'proxies' ? '把本地服务发布到服务器' : g.key === 'visitors' ? '访问私有代理服务' : '高级自定义配置' }}</small>
            </button>
          </div>

          <div v-if="selectedSection !== 'custom'" class="rail-group">
            <span class="rail-label">模板</span>
            <button
              v-for="template in currentSectionGroup.templates"
              :key="template.id"
              type="button"
              class="template-option"
              :class="{ active: selectedTemplateId === template.id }"
              @click="selectedTemplateId = template.id"
            >
              <strong>{{ template.label }}</strong>
              <small>{{ template.description }}</small>
            </button>
          </div>
        </aside>

        <section class="form-panel">
          <div class="form-panel-head">
            <div>
              <h2>{{ currentPreset.label }}</h2>
              <p>{{ currentPreset.description }}</p>
            </div>
            <span class="preset-type">{{ selectedSection === 'custom' ? customSectionName || 'custom' : selectedSection }}</span>
          </div>

          <div v-if="selectedSection === 'custom'" class="custom-name-row">
            <label class="field-card">
              <span class="field-label">段落名称<em class="req">*</em></span>
              <input v-model="customSectionName" class="input" type="text" placeholder="例如 my_section" />
            </label>
          </div>

          <div v-if="basicFields.length" class="field-section">
            <div class="field-section-head">
              <h3>常用参数</h3>
              <span>填完这些通常就能启动</span>
            </div>
            <div class="field-grid">
              <label
                v-for="field in basicFields"
                :key="field.key"
                class="field-card"
                :class="{ 'field-bool': field.kind === 'boolean' }"
              >
                <span class="field-label">
                  {{ field.label }}<em v-if="field.required" class="req">*</em>
                </span>
                <template v-if="field.kind === 'select'">
                  <select v-model="formValues[field.key]" class="input">
                    <option value="">请选择</option>
                    <option v-for="opt in field.options" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
                  </select>
                </template>
                <template v-else-if="field.kind === 'boolean'">
                  <label class="toggle">
                    <input v-model="formValues[field.key]" type="checkbox" />
                    <span>启用</span>
                  </label>
                </template>
                <template v-else>
                  <input
                    v-model="formValues[field.key]"
                    class="input"
                    :type="field.kind === 'number' ? 'number' : 'text'"
                    :placeholder="field.placeholder"
                  />
                </template>
                <small v-if="field.help" class="field-help">{{ field.help }}</small>
              </label>
            </div>
          </div>

          <details v-if="advancedFields.length" class="advanced-panel">
            <summary>高级参数</summary>
            <div class="field-grid">
              <label
                v-for="field in advancedFields"
                :key="field.key"
                class="field-card"
                :class="{ 'field-bool': field.kind === 'boolean' }"
              >
                <span class="field-label">
                  {{ field.label }}<em v-if="field.required" class="req">*</em>
                </span>
                <template v-if="field.kind === 'select'">
                  <select v-model="formValues[field.key]" class="input">
                    <option value="">请选择</option>
                    <option v-for="opt in field.options" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
                  </select>
                </template>
                <template v-else-if="field.kind === 'boolean'">
                  <label class="toggle">
                    <input v-model="formValues[field.key]" type="checkbox" />
                    <span>启用</span>
                  </label>
                </template>
                <template v-else>
                  <input
                    v-model="formValues[field.key]"
                    class="input"
                    :type="field.kind === 'number' ? 'number' : 'text'"
                    :placeholder="field.placeholder"
                  />
                </template>
                <small v-if="field.help" class="field-help">{{ field.help }}</small>
              </label>
            </div>
          </details>

          <div class="extra">
            <div class="extra-head">
              <div>
                <h3>额外属性</h3>
                <p>用于官方新增字段或当前模板未覆盖的配置。</p>
              </div>
              <button class="btn btn-xs btn-ghost" type="button" @click="addExtraField">添加属性</button>
            </div>
            <div v-if="extraFields.length" class="extra-list">
              <div v-for="f in extraFields" :key="f.id" class="extra-row">
                <input v-model="f.key" class="input input-sm" type="text" placeholder="属性名" />
                <select v-model="f.kind" class="input input-xs">
                  <option value="text">文本</option>
                  <option value="number">数字</option>
                  <option value="boolean">布尔</option>
                  <option value="array">数组</option>
                </select>
                <template v-if="f.kind === 'boolean'">
                  <select v-model="f.value" class="input input-xs">
                    <option value="true">true</option>
                    <option value="false">false</option>
                  </select>
                </template>
                <template v-else>
                  <input v-model="f.value" class="input input-sm" type="text" :placeholder="f.kind === 'array' ? 'a, b, c' : '值'" />
                </template>
                <button class="btn btn-xs btn-red" type="button" @click="removeExtraField(f.id)">删除</button>
              </div>
            </div>
          </div>

          <div v-if="formError" class="msg msg-err msg-sm">{{ formError }}</div>

          <div class="form-actions">
            <button class="btn btn-ghost" type="button" @click="resetForm()">重置</button>
            <button class="btn btn-blue" type="button" @click="appendSectionFromForm">追加到编辑器</button>
          </div>
        </section>
      </div>
    </article>

    <!-- ====== 说明 (Help) ====== -->
    <article v-if="activeView === 'help'" class="body">
      <section class="help-panel">
        <div class="help-top">
          <div>
            <h2>frp 下载与使用说明</h2>
            <p>说明内容来自应用内置 Markdown，打包后不依赖外部文档文件。</p>
          </div>
          <button class="btn btn-ghost" type="button" @click="loadHelp">刷新说明</button>
        </div>
        <div class="help-download">
          <span>当前下载链接</span>
          <a :href="downloadTarget.url" target="_blank" rel="noreferrer">{{ downloadTarget.url || '读取中...' }}</a>
        </div>
        <div class="help-content">
          <template v-for="(block, index) in renderedHelpBlocks" :key="index">
            <h1 v-if="block.type === 'heading' && block.level === 1">{{ block.text }}</h1>
            <h2 v-else-if="block.type === 'heading' && block.level === 2">{{ block.text }}</h2>
            <h3 v-else-if="block.type === 'heading'">{{ block.text }}</h3>
            <p v-else-if="block.type === 'paragraph'">{{ block.text }}</p>
            <ul v-else-if="block.type === 'list'">
              <li v-for="item in block.items" :key="item">{{ item }}</li>
            </ul>
            <pre v-else-if="block.type === 'code'"><code>{{ block.text }}</code></pre>
          </template>
        </div>
      </section>
    </article>

    <!-- ====== 下载面板 (Download) ====== -->
    <article v-if="activeView === 'download'" class="body">
      <section class="dl-panel">
        <div class="dl-top">
          <h2>下载管理</h2>
          <div class="dl-top-btns">
            <button class="btn btn-sm btn-ghost" @click="refreshDlTasks">刷新</button>
            <button class="btn btn-sm btn-red" @click="removeCompleted">清除已完成</button>
          </div>
        </div>

        <div class="dl-form">
          <div class="dl-form-row">
            <input v-model="dlUrl" class="input dl-url" type="text" placeholder="输入下载链接 https://..." @keyup.enter="startDl" />
          </div>
          <div class="dl-form-row">
            <div class="path-row dl-path-row">
              <input v-model="dlDir" class="input" type="text" placeholder="保存目录" @keyup.enter="startDl" />
              <button class="btn btn-ghost" type="button" @click="pickDir">选择</button>
            </div>
          </div>
          <div class="dl-form-row">
            <div class="dl-fn-row">
              <input v-model="dlFilename" class="input dl-fn" type="text" placeholder="文件名" @keyup.enter="startDl" />
              <div class="dl-opts">
                <span class="dl-opt-label">连接数</span>
                <input v-model.number="dlConnections" class="input input-xs dl-num" type="number" min="1" max="32" />
              </div>
            </div>
            <button class="btn btn-blue" @click="startDl" :disabled="!dlUrl || !dlDir || !dlFilename">开始下载</button>
          </div>
        </div>

        <div class="dl-filter-bar">
          <button class="dl-filter-btn" :class="{ active: dlFilter === 'active' }" @click="dlFilter = 'active'">进行中 ({{ dlFilterCounts.active }})</button>
          <button class="dl-filter-btn" :class="{ active: dlFilter === 'done' }" @click="dlFilter = 'done'">已完成 ({{ dlFilterCounts.done }})</button>
          <button class="dl-filter-btn" :class="{ active: dlFilter === 'canceled' }" @click="dlFilter = 'canceled'">已取消 ({{ dlFilterCounts.canceled }})</button>
          <button class="dl-filter-btn" :class="{ active: dlFilter === 'error' }" @click="dlFilter = 'error'">失败 ({{ dlFilterCounts.error }})</button>
        </div>

        <div v-if="!filteredDlTasks.length" class="dl-empty">
          {{ dlFilter === 'active' ? '暂无进行中的下载任务' : dlFilter === 'done' ? '暂无已完成的任务' : dlFilter === 'canceled' ? '暂无已取消的任务' : '暂无失败的任务' }}
        </div>

        <div v-for="task in filteredDlTasks" :key="task.id" class="dl-task">
          <div class="dl-task-head">
            <span class="dl-task-state" :class="'dl-state-' + task.state">{{ task.state }}</span>
            <span class="dl-task-url" :title="task.url">{{ task.url }}</span>
            <span class="dl-task-path" :title="task.destPath">{{ task.destPath.split('/').pop() || task.destPath.split('\\').pop() || task.destPath }}</span>
            <button v-if="task.state === 'downloading'" class="btn btn-xs btn-red" @click="cancelDl(task.id)">停止</button>
          </div>
          <div class="dl-task-bar">
            <div class="dl-task-fill" :style="{ width: task.progress.percentage + '%' }" :class="{ 'dl-done': task.state === 'done', 'dl-err': task.state === 'error' }"></div>
          </div>
          <div class="dl-task-info">
            <span>{{ formatBytes(task.progress.downloaded) }} / {{ task.progress.total > 0 ? formatBytes(task.progress.total) : '未知大小' }}</span>
            <span v-if="task.state === 'downloading' && task.progress.speed > 0">{{ formatBytes(task.progress.speed) }}/s</span>
            <span v-if="task.state === 'downloading' && task.progress.speed > 0 && task.progress.total > 0">剩余 {{ formatEta(task.progress.downloaded, task.progress.total, task.progress.speed) }}</span>
            <span v-if="task.state === 'done'">{{ task.progress.percentage.toFixed(1) }}%</span>
            <span v-if="task.state === 'error'" class="dl-err-text">{{ task.progress.error }}</span>
          </div>
        </div>
      </section>
    </article>

    <!-- ====== 状态 (Status) ====== -->
    <article v-if="activeView === 'status'" class="body">
      <section class="st-panel">
        <div class="st-top">
          <h2>frpc 运行状态</h2>
          <button class="btn btn-sm btn-ghost" @click="refreshFrpStatus">刷新</button>
        </div>
        <div class="st-status-line">
          <span class="status-dot" :class="{ on: isRunning }"></span>
          <span class="st-status-label">{{ isRunning ? '运行中' : '已停止' }}</span>
          <span v-if="isRunning && frpStatus.pid" class="st-pid">PID {{ frpStatus.pid }}</span>
          <span v-if="isRunning && frpStatus.uptime" class="st-uptime">运行 {{ frpStatus.uptime }}</span>
        </div>
        <div v-if="frpcInstalled" class="st-grid">
          <div class="st-item"><span class="st-key">frpc 版本</span><span class="st-val">{{ frpcVersion || '--' }}</span></div>
          <div class="st-item"><span class="st-key">系统</span><span class="st-val">{{ systemInfo?.os || '--' }} {{ systemInfo?.arch || '' }}</span></div>
          <div class="st-item st-item-wide"><span class="st-key">工具路径</span><span class="st-val">{{ toolPath || '--' }}</span></div>
          <div class="st-item st-item-wide"><span class="st-key">配置路径</span><span class="st-val">{{ configPath || '--' }}</span></div>
        </div>
        <div class="st-grid">
          <div class="st-item"><span class="st-key">服务器</span><span class="st-val">{{ serverSummary }}</span></div>
          <div class="st-item"><span class="st-key">认证</span><span class="st-val">{{ authSummary }}</span></div>
          <div class="st-item"><span class="st-key">代理数</span><span class="st-val">{{ proxyCount }}</span></div>
          <div class="st-item"><span class="st-key">访问器数</span><span class="st-val">{{ visitorCount }}</span></div>
        </div>
      </section>
    </article>

    <!-- ====== 日志 (Logs) ====== -->
    <article v-if="activeView === 'logs'" class="body-log">
      <div class="log-head">
        <span class="log-title">frpc 日志</span>
        <button class="btn btn-xs btn-ghost" @click="refreshLogs">刷新</button>
      </div>
      <div class="log-body log-body-full">
        <div v-if="!renderedLogLines.length" class="log-empty">(暂无日志)</div>
        <div v-for="line in renderedLogLines" :key="line.id" class="log-line" :class="`log-line-${line.level}`">{{ line.content }}</div>
      </div>
    </article>

    <!-- ====== 设置 (Settings) ====== -->
    <article v-if="activeView === 'settings'" class="body">
      <section class="st-panel">
        <div class="st-top">
          <h2>设置</h2>
        </div>
        <div class="settings-stack">
          <label class="settings-field">
            <span>主题</span>
            <div class="segmented">
              <button type="button" :class="{ active: settingsDraft.theme === 'dark' }" @click="settingsDraft.theme = 'dark'">黑夜</button>
              <button type="button" :class="{ active: settingsDraft.theme === 'light' }" @click="settingsDraft.theme = 'light'">白天</button>
            </div>
          </label>
          <label class="settings-field">
            <span>工具路径</span>
            <div class="path-row">
              <input v-model="settingsDraft.toolPath" class="input" type="text" placeholder="~/frp-client/frpc" />
              <button class="btn btn-ghost" type="button" @click="pickToolPath">选择</button>
            </div>
            <small v-if="settingsFileStatus && !settingsFileStatus.toolExists" class="field-help warning-help">{{ settingsFileStatus.toolHelp }}</small>
          </label>
          <label class="settings-field">
            <span>配置路径</span>
            <div class="path-row">
              <input v-model="settingsDraft.configPath" class="input" type="text" placeholder="~/frp-client/frpc.toml" />
              <button class="btn btn-ghost" type="button" @click="pickConfigPath">选择</button>
            </div>
            <small v-if="settingsFileStatus && !settingsFileStatus.configExists" class="field-help warning-help">{{ settingsFileStatus.configHelp }}</small>
          </label>
          <label class="settings-field">
            <span>下载链接</span>
            <input v-model="settingsDraft.downloadUrl" class="input" type="text" placeholder="https://github.com/fatedier/frp/releases/download/{tag}/{filename}" />
            <small class="field-help">支持 {tag}、{version}、{filename} 占位符。</small>
          </label>
          <label class="settings-toggle">
            <input v-model="settingsDraft.autoStart" type="checkbox" />
            <span class="toggle-visual"></span>
            <span class="toggle-copy">
              <strong>开机自启动 frpc 服务</strong>
              <small>使用工具路径中的 frpc，并加载配置路径中的 frpc.toml。</small>
            </span>
          </label>
          <div v-if="settingsFileStatus" class="settings-note">{{ settingsFileStatus.downloadHelp }}</div>
        </div>
        <div v-if="settingsError" class="msg msg-err msg-sm">{{ settingsError }}</div>
        <div class="form-actions">
          <button class="btn btn-red" type="button" @click="resetAppSettings">重置默认设置</button>
          <button class="btn btn-blue" type="button" @click="saveAppSettings">保存设置</button>
        </div>
      </section>
    </article>
  </div>

  <!-- 消息 -->
  <div v-if="actionError || actionSuccess || duplicateWarnings.length" class="msg-bar">
    <div v-if="actionError" class="msg msg-err">
      <span>{{ actionError }}</span>
      <button class="msg-close" @click="actionError = ''">✕</button>
    </div>
    <div v-if="actionSuccess" class="msg msg-ok">
      <span>{{ actionSuccess }}</span>
      <button class="msg-close" @click="actionSuccess = ''">✕</button>
    </div>
    <div v-if="duplicateWarnings.length" class="msg msg-warn">
      <span>{{ duplicateWarnings.join('；') }}</span>
      <button class="msg-close" @click="duplicateWarnings.splice(0)">✕</button>
    </div>
  </div>
</template>

<style scoped>
* {
  box-sizing: border-box;
}

.app-container {
  --bg: #0f1419;
  --surface: #161b22;
  --surface-2: #0d1117;
  --surface-3: #21262d;
  --surface-soft: #121820;
  --border: #30363d;
  --border-muted: #21262d;
  --text: #c9d1d9;
  --text-strong: #f0f6fc;
  --muted: #8b949e;
  --muted-2: #6e7681;
  --accent: #2f81f7;
  --accent-2: #f78166;
  --ok: #3fb950;
  --danger: #f85149;
  --warn: #d29922;
  --log-info: #58a6ff;
  --log-warn: #d29922;
  --log-error: #ff7b72;
  --log-debug: #a371f7;
  display: flex;
  flex-direction: column;
  height: 100vh;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Segoe UI', Roboto, sans-serif;
  background: var(--bg);
  color: var(--text);
}

.app-container.theme-light {
  --bg: #eef2f7;
  --surface: #ffffff;
  --surface-2: #f8fafc;
  --surface-3: #e2e8f0;
  --surface-soft: #f1f5f9;
  --border: #cbd5e1;
  --border-muted: #e2e8f0;
  --text: #243041;
  --text-strong: #0f172a;
  --muted: #64748b;
  --muted-2: #94a3b8;
  --accent: #2563eb;
  --accent-2: #0f766e;
  --ok: #16833a;
  --danger: #dc2626;
  --warn: #b7791f;
  --log-info: #2563eb;
  --log-warn: #b7791f;
  --log-error: #dc2626;
  --log-debug: #7c3aed;
}

/* ---- header ---- */
.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 24px;
  height: 48px;
  background: var(--surface);
  border-bottom: 1px solid var(--border-muted);
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h1 {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
  color: var(--text-strong);
  letter-spacing: 0.02em;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--muted-2);
}
.status-dot.on {
  background: var(--ok);
  box-shadow: 0 0 6px rgba(63, 185, 80, 0.4);
}

.status-text {
  font-size: 12px;
  color: var(--muted);
}

.uptime {
  font-size: 11px;
  color: var(--muted-2);
}

.header-right {
  display: flex;
  gap: 6px;
}

/* ---- panels ---- */
.panel {
  padding: 12px 24px;
  background: var(--surface-2);
  border-bottom: 1px solid var(--border-muted);
  flex-shrink: 0;
}

.panel-row {
  display: flex;
  gap: 12px;
  align-items: flex-end;
}

.panel-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 180px;
}

.panel-cell:nth-child(2) {
  flex: 1;
}

.download-head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: end;
  margin-bottom: 10px;
}

.download-head code {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: var(--text);
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.download-file {
  padding: 4px 8px;
  border: 1px solid var(--border-muted);
  border-radius: 999px;
  color: var(--muted);
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 11px;
}

.command-input {
  width: 100%;
  font-family: 'SF Mono', 'Menlo', monospace;
}

.panel-btn {
  flex-direction: row;
  gap: 8px;
  justify-content: flex-end;
}

.panel-log {
  padding: 14px 24px;
}

.log-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.log-title {
  font-size: 12px;
  font-weight: 600;
  color: #8b949e;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.log-body {
  margin: 0;
  max-height: 180px;
  overflow-y: auto;
  background: var(--surface-2);
  border: 1px solid var(--border-muted);
  border-radius: 6px;
  padding: 10px 14px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 11px;
  line-height: 1.6;
  color: var(--muted);
  white-space: pre-wrap;
  word-break: break-all;
}

.log-empty {
  color: var(--muted-2);
}

.log-line {
  min-height: 17px;
}

.log-line-info {
  color: var(--log-info);
}

.log-line-warn {
  color: var(--log-warn);
}

.log-line-error {
  color: var(--log-error);
}

.log-line-debug {
  color: var(--log-debug);
}

.progress {
  margin-top: 8px;
  height: 3px;
  background: var(--surface-3);
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: var(--ok);
  transition: width 0.3s ease;
}

/* ---- messages ---- */
.msg {
  padding: 7px 14px;
  font-size: 12px;
  flex-shrink: 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.msg-err {
  background: #490202;
  color: #ffb4ad;
  border-bottom: 1px solid #660303;
}
.msg-ok {
  background: #04260f;
  color: #7ee787;
  border-bottom: 1px solid #0c4220;
}
.msg-warn {
  background: #332701;
  color: #f2cc60;
  border-bottom: 1px solid #4d3a03;
}
.theme-light .msg-err {
  background: #fee2e2;
  color: #991b1b;
  border-bottom-color: #fecaca;
}
.theme-light .msg-ok {
  background: #dcfce7;
  color: #166534;
  border-bottom-color: #bbf7d0;
}
.theme-light .msg-warn {
  background: #fef3c7;
  color: #92400e;
  border-bottom-color: #fde68a;
}
.msg-sm {
  padding: 6px 0;
  margin-bottom: 10px;
  border-radius: 4px;
  padding-left: 10px;
  border: none;
}

/* ---- overview ---- */
.overview {
  display: grid;
  grid-template-columns: 1.4fr 1fr 0.5fr 0.5fr 2fr;
  gap: 1px;
  grid-template-columns: 1.2fr 0.9fr 0.4fr 0.4fr 1.6fr 1.6fr;
  background: var(--border-muted);
  border-bottom: 1px solid var(--border-muted);
  flex-shrink: 0;
}

.metric {
  min-width: 0;
  padding: 10px 16px;
  background: var(--surface-2);
}

.metric-label {
  display: block;
  margin-bottom: 3px;
  color: var(--muted-2);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.metric strong {
  display: block;
  overflow: hidden;
  color: var(--text);
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 12px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---- tabs ---- */
.tabs {
  display: flex;
  padding: 0 24px;
  background: var(--surface);
  border-bottom: 1px solid var(--border-muted);
  flex-shrink: 0;
}

.tab {
  padding: 10px 18px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.tab:hover {
  color: var(--text);
}
.tab.active {
  color: var(--text-strong);
  border-bottom-color: var(--accent-2);
}

/* ---- body ---- */
.body {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

/* ---- editor ---- */
.editor-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.editor-bar-left {
  display: flex;
  gap: 8px;
}
.editor-path {
  font-size: 11px;
  color: var(--muted-2);
  font-family: 'SF Mono', 'Menlo', monospace;
}

.editor-area {
  width: 100%;
  height: calc(100vh - 220px);
  min-height: 300px;
  background: var(--surface-2);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 16px;
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.7;
  resize: none;
  outline: none;
  tab-size: 2;
}
.editor-area:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.15);
}

/* ---- browse ---- */
.empty {
  color: var(--muted-2);
  text-align: center;
  padding: 60px 0;
  font-size: 14px;
}

.block {
  margin-bottom: 28px;
}

.block-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 10px;
}

.block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--accent-2);
  font-family: 'SF Mono', 'Menlo', monospace;
}

.block-subtitle {
  margin-top: 3px;
  color: var(--muted-2);
  font-size: 11px;
}

.table-wrap {
  overflow-x: auto;
  border: 1px solid var(--border-muted);
  border-radius: 6px;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

th, td {
  padding: 7px 12px;
  text-align: left;
  white-space: nowrap;
}

th {
  background: var(--surface);
  color: var(--muted);
  font-weight: 500;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

td {
  color: var(--text);
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  border-top: 1px solid var(--border-muted);
}

tbody tr {
  transition: background 0.1s;
}
tbody tr:hover {
  background: var(--surface);
}

.actions-col {
  width: 160px;
  text-align: right;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  min-width: 160px;
}

/* ---- add form ---- */
.add-workspace {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

.template-rail,
.form-panel {
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface);
}

.template-rail {
  position: sticky;
  top: 0;
  display: grid;
  gap: 20px;
  padding: 16px;
  max-height: calc(100vh - 220px);
  overflow-y: auto;
}

.rail-group {
  display: grid;
  gap: 8px;
}

.rail-label,
.field-section-head span,
.preset-type {
  color: var(--muted-2);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.template-option {
  display: grid;
  gap: 4px;
  width: 100%;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  text-align: left;
}

.template-option strong {
  font-size: 13px;
  color: var(--text-strong);
}

.template-option small {
  color: var(--muted);
  line-height: 1.45;
}

.template-option:hover,
.template-option.active {
  border-color: rgba(47, 129, 247, 0.35);
  background: rgba(47, 129, 247, 0.1);
}

.form-panel {
  padding: 20px;
}

.form-panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--border-muted);
}

.form-panel-head h2 {
  margin: 0;
  color: var(--text-strong);
  font-size: 20px;
  line-height: 1.2;
}

.form-panel-head p,
.extra-head p {
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.preset-type {
  flex: 0 0 auto;
  padding: 5px 8px;
  border: 1px solid var(--border-muted);
  border-radius: 999px;
  background: var(--surface-2);
}

.field-section,
.custom-name-row {
  margin-top: 18px;
}

.field-section-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 10px;
}

.field-section-head h3,
.extra-head h3 {
  margin: 0;
  color: var(--text-strong);
  font-size: 13px;
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(220px, 1fr));
  gap: 12px;
}

.field-card {
  display: grid;
  gap: 7px;
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface-2);
}

.field-label {
  font-size: 12px;
  color: var(--muted);
  font-weight: 600;
}
.req {
  color: var(--danger);
  font-style: normal;
  margin-left: 2px;
}

.field-bool {
  align-content: start;
}

.field-help {
  font-size: 11px;
  color: var(--muted-2);
  line-height: 1.45;
}

.advanced-panel {
  margin-top: 18px;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface-2);
}

.advanced-panel summary {
  padding: 12px 14px;
  color: var(--text-strong);
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.advanced-panel .field-grid {
  padding: 0 14px 14px;
}

/* ---- help ---- */
.help-panel {
  max-width: 980px;
  margin: 0 auto;
}

.help-top,
.help-download {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-muted);
}

.help-top h2 {
  margin: 0 0 4px;
  color: var(--text-strong);
  font-size: 20px;
}

.help-top p {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.help-download {
  margin-top: 14px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface-2);
}

.help-download span {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 12px;
}

.help-download a {
  min-width: 0;
  overflow: hidden;
  color: var(--log-info);
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.help-content {
  display: grid;
  gap: 10px;
  padding: 18px 0 40px;
}

.help-content h1,
.help-content h2,
.help-content h3 {
  margin: 12px 0 2px;
  color: var(--text-strong);
}

.help-content h1 { font-size: 22px; }
.help-content h2 { font-size: 17px; }
.help-content h3 { font-size: 14px; }

.help-content p,
.help-content li {
  margin: 0;
  color: var(--text);
  font-size: 13px;
  line-height: 1.7;
}

.help-content ul {
  margin: 0;
  padding-left: 20px;
}

.help-content pre {
  margin: 0;
  overflow-x: auto;
  padding: 12px 14px;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--text);
  font-family: 'SF Mono', 'Menlo', monospace;
  font-size: 12px;
  line-height: 1.6;
}

/* ---- labels / inputs ---- */
.label {
  font-size: 11px;
  color: var(--muted);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.input, select {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}
.input:focus, select:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.15);
}
.input-sm { min-width: 150px; }
.input-xs { min-width: 100px; }

.toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text);
  cursor: pointer;
}
.toggle input[type="checkbox"] {
  accent-color: var(--accent);
}

/* ---- extra fields ---- */
.extra {
  margin-top: 20px;
  padding-top: 14px;
  border-top: 1px solid var(--border-muted);
}
.extra-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.extra-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.extra-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  background: var(--surface-2);
}

/* ---- buttons ---- */
.btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid var(--border);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  background: var(--surface-3);
  color: var(--text);
}
.btn:hover { background: var(--border); }
.btn:disabled { opacity: 0.4; cursor: default; }

.btn-sm { padding: 4px 10px; font-size: 11px; }
.btn-xs { padding: 3px 8px; font-size: 11px; border-radius: 4px; }

.btn-blue {
  background: var(--accent);
  border-color: var(--accent);
  color: #fff;
}
.btn-blue:hover { background: #388bfd; }

.btn-green {
  background: var(--ok);
  border-color: var(--ok);
  color: #fff;
}
.btn-green:hover { background: #2ea043; }

.btn-red {
  background: var(--danger);
  border-color: var(--danger);
  color: #fff;
}
.btn-red:hover { background: #f85149; }

.btn-ghost {
  background: transparent;
  border-color: transparent;
  color: var(--muted);
}
.btn-ghost:hover {
  background: var(--surface-3);
  color: var(--text);
}

.form-actions {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.settings-stack {
  display: grid;
  gap: 16px;
  margin-top: 18px;
}

.settings-field {
  display: grid;
  gap: 8px;
}

.settings-field > span {
  color: var(--muted);
  font-size: 12px;
  font-weight: 700;
}

.warning-help {
  color: var(--warn);
}

.path-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.segmented {
  display: inline-flex;
  width: fit-content;
  padding: 3px;
  border: 1px solid var(--border-muted);
  border-radius: 6px;
  background: var(--surface-2);
}

.segmented button {
  min-width: 72px;
  padding: 6px 12px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.segmented button.active {
  background: var(--accent);
  color: #fff;
}

.settings-toggle {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  align-items: center;
  padding: 12px;
  border: 1px solid var(--border-muted);
  border-radius: 6px;
  background: var(--surface-2);
  cursor: pointer;
}

.settings-toggle input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.toggle-visual {
  position: relative;
  width: 38px;
  height: 22px;
  border-radius: 999px;
  background: var(--surface-3);
  border: 1px solid var(--border);
}

.toggle-visual::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--muted);
  transition: transform 0.16s ease, background 0.16s ease;
}

.settings-toggle input:checked + .toggle-visual {
  background: rgba(47, 129, 247, 0.18);
  border-color: var(--accent);
}

.settings-toggle input:checked + .toggle-visual::after {
  transform: translateX(16px);
  background: var(--accent);
}

.toggle-copy {
  display: grid;
  gap: 3px;
}

.toggle-copy strong {
  color: var(--text-strong);
  font-size: 13px;
}

.toggle-copy small {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.settings-note {
  padding: 10px 12px;
  border: 1px solid var(--border-muted);
  border-radius: 6px;
  color: var(--muted);
  background: var(--surface-2);
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .app-header {
    height: auto;
    min-height: 48px;
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
    padding: 10px 16px;
  }

  .header-right {
    flex-wrap: wrap;
  }

  .overview,
  .add-workspace,
  .field-grid {
    grid-template-columns: 1fr;
  }

  .metric-wide {
    grid-column: auto;
  }

  .body {
    padding: 16px;
  }

  .editor-bar,
  .panel-row,
  .path-row,
  .extra-row {
    align-items: stretch;
    flex-direction: column;
    display: flex;
  }

  .field-label {
    width: auto;
    text-align: left;
  }

  .input-sm,
  .input-xs {
    width: 100%;
    max-width: none;
  }
}

/* ====== 下载面板 ====== */

.dl-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dl-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.dl-top h2 {
  font-size: 16px;
  font-weight: 700;
  margin: 0;
}

.dl-top-btns {
  display: flex;
  gap: 8px;
}

.dl-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface);
}

.dl-form-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.dl-url {
  flex: 1;
}

.dl-path-row {
  flex: 1;
}

.dl-opts {
  display: flex;
  align-items: center;
  gap: 6px;
}

.dl-opt-label {
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.dl-num {
  width: 56px;
  text-align: center;
}

.dl-empty {
  padding: 32px;
  text-align: center;
  color: var(--muted);
  font-size: 13px;
}

.dl-task {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--border-muted);
  border-radius: 6px;
  background: var(--surface);
}

.dl-task-head {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 12px;
}

.dl-task-state {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  flex-shrink: 0;
}

.dl-state-pending { background: var(--surface-3); color: var(--muted); }
.dl-state-downloading { background: rgba(47,129,247,0.15); color: var(--accent); }
.dl-state-done { background: rgba(63,185,80,0.15); color: var(--ok); }
.dl-state-error { background: rgba(248,81,73,0.15); color: var(--danger); }
.dl-state-canceled { background: rgba(210,153,34,0.15); color: var(--warn); }

.dl-task-url {
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.dl-task-path {
  color: var(--muted);
  flex-shrink: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}

.dl-task-bar {
  height: 6px;
  border-radius: 3px;
  background: var(--surface-3);
  overflow: hidden;
}

.dl-task-fill {
  height: 100%;
  border-radius: 3px;
  background: var(--accent);
  transition: width 0.3s;
  min-width: 0;
}

.dl-task-fill.dl-done { background: var(--ok); }
.dl-task-fill.dl-err { background: var(--danger); }

.dl-task-info {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--muted);
}

.dl-err-text {
  color: var(--danger);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.st-panel { display: flex; flex-direction: column; gap: 16px; }
.st-top { display: flex; justify-content: space-between; align-items: center; }
.st-top h2 { font-size: 16px; font-weight: 700; margin: 0; }
.st-status-line { display: flex; align-items: center; gap: 12px; padding: 12px; border-radius: 8px; background: var(--surface); border: 1px solid var(--border-muted); }
.st-status-label { font-weight: 700; font-size: 15px; }
.st-pid { color: var(--muted); font-size: 13px; }
.st-uptime { color: var(--muted); font-size: 13px; margin-left: auto; }
.st-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.st-item { display: flex; flex-direction: column; gap: 3px; padding: 10px 12px; border-radius: 6px; background: var(--surface); border: 1px solid var(--border-muted); }
.st-item-wide { grid-column: 1 / -1; }
.st-key { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; }
.st-val { font-size: 13px; color: var(--text-strong); word-break: break-all; }

.log-panel { display: flex; flex-direction: column; gap: 12px; }
.dl-filter-bar { display: flex; gap: 4px; }
.dl-filter-btn { padding: 4px 12px; border: 1px solid var(--border-muted); border-radius: 6px; background: var(--surface-2); color: var(--muted); font-size: 12px; cursor: pointer; transition: all 0.15s; }
.dl-filter-btn:hover { border-color: var(--border); color: var(--text); }
.dl-filter-btn.active { background: var(--accent); border-color: var(--accent); color: #fff; }
.dl-fn-row { display: flex; gap: 8px; align-items: center; flex: 1; }
.dl-fn { flex: 1; }

.body-log { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-height: 0; padding: 24px 24px 0; }
.log-body-full { flex: 1; overflow-y: auto; margin: 0 -24px; padding: 0 24px; max-height: none; }

.msg-bar { position: fixed; bottom: 0; left: 0; right: 0; z-index: 100; display: flex; flex-direction: column; gap: 4px; padding: 8px; }
.msg-close { background: transparent; border: none; color: inherit; opacity: 0.6; cursor: pointer; font-size: 14px; padding: 0 4px; line-height: 1; flex-shrink: 0; }
.msg-close:hover { opacity: 1; }
</style>
