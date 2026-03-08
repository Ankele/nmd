<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import MarkdownIt from "markdown-it";
import markdownItTaskLists from "markdown-it-task-lists";
import hljs from "highlight.js";
import "highlight.js/styles/github.css";
import usageMarkdown from "./usage.md?raw";
import { Compartment, EditorSelection, EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, redo, undo } from "@codemirror/commands";
import { markdown } from "@codemirror/lang-markdown";
import { oneDark } from "@codemirror/theme-one-dark";
import {
  AddRecentFile,
  ClearRecentFiles,
  ExportMarkdownAsPDF,
  ListRecentFiles,
  LoadImageDataURL,
  OpenMarkdownFile,
  OpenMarkdownFileAtPath,
  RemoveRecentFile,
  ResolveImageDataURL,
  SaveImageAsset,
  SetDirtyState,
  SaveMarkdownFile,
} from "../wailsjs/go/main/App";
import {
  BrowserOpenURL,
  OnFileDrop,
  OnFileDropOff,
  WindowIsMaximised,
  WindowSetTitle,
  WindowToggleMaximise,
} from "../wailsjs/runtime/runtime";

const RECENT_FILES_KEY = "nmd.recentFiles.browser";
const RECENT_ORDER_KEY = "nmd.recentFiles.order";
const RECENT_PINNED_KEY = "nmd.recentFiles.pinned";
const DRAFT_CONTENT_KEY = "nmd.draft.content";
const DRAFT_NAME_KEY = "nmd.draft.name";
const DRAFT_PATH_KEY = "nmd.draft.path";
const LAST_FILE_PATH_KEY = "nmd.lastFile.path";
const UI_THEME_KEY = "nmd.ui.theme";
const UI_SIDEBAR_KEY = "nmd.ui.sidebar";
const UI_VIEWMODE_KEY = "nmd.ui.viewmode";
const UI_SPLIT_RATIO_KEY = "nmd.ui.splitRatio";
const UI_SIDEBAR_WIDTH_KEY = "nmd.ui.sidebarWidth";
const UI_OUTLINE_COLLAPSE_KEY = "nmd.ui.outlineCollapsed";
const UI_ZEN_KEY = "nmd.ui.zen";
const UI_SCROLL_SYNC_KEY = "nmd.ui.scrollSync";
const UI_LINE_NUMBERS_KEY = "nmd.ui.lineNumbers";
const UI_WRAP_LINES_KEY = "nmd.ui.wrapLines";
const UI_EDITOR_FONT_SIZE_KEY = "nmd.ui.editorFontSize";
const UI_EDITOR_FONT_FAMILY_KEY = "nmd.ui.editorFontFamily";
const UI_STATUSBAR_KEY = "nmd.ui.statusbar";
const UI_AUTOSAVE_KEY = "nmd.ui.autosave";
const UI_AUTOSAVE_INTERVAL_KEY = "nmd.ui.autosaveInterval";
const UI_AUTOSAVE_ERRORS_KEY = "nmd.ui.autosaveErrors";
const UI_AUTOSAVE_ERROR_SOURCE_KEY = "nmd.ui.autosaveError.source";
const UI_AUTOSAVE_ERROR_QUERY_KEY = "nmd.ui.autosaveError.query";
const UI_AUTOSAVE_ERROR_SORT_KEY = "nmd.ui.autosaveError.sort";
const UI_LANG_KEY = "nmd.ui.lang";
const UI_SHORTCUT_REDO_Y_KEY = "nmd.ui.shortcut.redoY";
const UI_SHORTCUT_ZEN_KEY = "nmd.ui.shortcut.zen";
const UI_SHORTCUT_BINDINGS_KEY = "nmd.ui.shortcut.bindings";
const DEFAULT_SPLIT_RATIO = 50;
const DEFAULT_SIDEBAR_WIDTH = 240;
const AUTOSAVE_INTERVAL_OPTIONS = [1200, 2500, 5000] as const;
const EDITOR_FONT_FAMILY_OPTIONS = [
  { value: '"JetBrains Mono", "SF Mono", "Menlo", monospace', label: "JetBrains Mono" },
  { value: '"Fira Code", "SF Mono", "Menlo", monospace', label: "Fira Code" },
  { value: '"Source Code Pro", "SF Mono", "Menlo", monospace', label: "Source Code Pro" },
  { value: '"LXGW WenKai Mono", "PingFang SC", "Microsoft YaHei", monospace', label: "WenKai Mono" },
] as const;

type RecentFile = {
  path: string;
  name: string;
};

type OutlineItem = {
  id: string;
  level: number;
  title: string;
  pos: number;
  line: number;
};

type VisibleOutlineItem = OutlineItem & {
  hasChildren: boolean;
  collapsed: boolean;
};

type AutosaveErrorEntry = {
  id: string;
  at: string;
  source: "autosave" | "save" | "saveAs";
  message: string;
};

type AutosaveErrorSourceFilter = "all" | AutosaveErrorEntry["source"];
type AutosaveErrorSortOrder = "desc" | "asc";
type UILanguage = "zh" | "en";
type ShortcutBindingKey = "commandPalette" | "help" | "settings" | "usage" | "zen";
type ShortcutPattern = {
  primary: boolean;
  shift: boolean;
  alt: boolean;
  key: string;
};
const DEFAULT_SHORTCUT_BINDINGS: Record<ShortcutBindingKey, string> = {
  commandPalette: "Ctrl/Cmd+K",
  help: "Ctrl/Cmd+/",
  settings: "Ctrl/Cmd+,",
  usage: "Ctrl/Cmd+Shift+U",
  zen: "Ctrl/Cmd+Shift+X",
};

type DocTab = {
  id: string;
  content: string;
  name: string;
  path: string;
  dirty: boolean;
};

type ExportedSettings = {
  version: 1;
  theme: "dark" | "light";
  language: UILanguage;
  showSidebar: boolean;
  showStatusbar: boolean;
  showLineNumbers: boolean;
  wrapLines: boolean;
  scrollSync: boolean;
  editorFontSize: number;
  editorFontFamily: string;
  autosaveEnabled: boolean;
  autosaveIntervalMs: number;
  enableRedoWithY: boolean;
  enableZenShortcut: boolean;
  shortcutBindings: Record<ShortcutBindingKey, string>;
};

type Command =
  | "new"
  | "open"
  | "save"
  | "saveAs"
  | "export"
  | "exportPdf"
  | "undo"
  | "redo"
  | "find"
  | "gotoLine"
  | "replace"
  | "replaceAll"
  | "toggleTheme"
  | "toggleLanguage"
  | "settings"
  | "showUsage"
  | "toggleSidebar"
  | "toggleZen"
  | "toggleScrollSync"
  | "toggleLineNumbers"
  | "toggleWrapLines"
  | "toggleStatusbar"
  | "toggleAutosave"
  | "cycleAutosaveInterval"
  | "retryAutosave"
  | "showAutosaveError"
  | "copyAutosaveError"
  | "exportAutosaveErrorLog"
  | "clearAutosaveError"
  | "undoAutosaveErrorDelete"
  | "redoAutosaveErrorDelete"
  | "fontSmaller"
  | "fontLarger"
  | "fontReset"
  | "resetLayout"
  | "viewSplit"
  | "viewEditOnly"
  | "viewPreviewOnly"
  | "toggleMaximise"
  | "help"
  | "fmtBold"
  | "fmtItalic"
  | "fmtCode"
  | "fmtH1"
  | "fmtH2"
  | "fmtQuote"
  | "fmtBullet"
  | "palette";

const escapeHtml = (unsafe: string): string =>
  unsafe
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#039;");

const escapeRegExp = (input: string): string => input.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

const I18N: Record<UILanguage, Record<string, string>> = {
  zh: {
    sidebar: "侧边栏",
    split: "分栏",
    edit: "编辑",
    preview: "预览",
    restore: "还原",
    maximize: "最大化",
    new: "新建",
    open: "打开",
    save: "保存",
    saveAs: "另存为",
    autosave: "自动保存",
    auto: "自动",
    on: "开",
    off: "关",
    utf8: "UTF-8",
    lineCol: "行 {line}, 列 {col}",
    words: "{count} 词",
    chars: "{count} 字符",
    minRead: "{count} 分钟阅读",
    autosaveIdle: "自动保存：空闲",
    autosavePending: "自动保存：等待中",
    autosaveSaving: "自动保存：保存中...",
    autosaveFailed: "自动保存：失败",
    autosaveSaved: "自动保存：已保存",
    autosaveSavedAt: "自动保存：{at} 已保存",
    cycleAutosaveInterval: "切换自动保存间隔",
    showAutosaveError: "显示自动保存错误",
    copyAutosaveError: "复制自动保存错误",
    exportAutosaveErrorLog: "导出自动保存错误日志",
    clearAutosaveError: "清空自动保存错误",
    findReplace: "查找替换",
    goToLine: "跳转到行",
    pdf: "PDF",
    shortcuts: "快捷键",
    sync: "同步",
    lineNo: "行号",
    wrap: "换行",
    bar: "状态栏",
    zen: "专注",
    light: "浅色",
    dark: "深色",
    command: "命令",
    bold: "加粗",
    italic: "斜体",
    quote: "引用",
    list: "列表",
    code: "代码",
    codeBlock: "代码块",
    editorFontMinus: "编辑器字体 -",
    editorFontPlus: "编辑器字体 +",
    editorFontReset: "编辑器字体重置",
    find: "查找",
    replaceWith: "替换为",
    matchCase: "区分大小写",
    findNext: "下一个",
    replace: "替换",
    replaceAll: "全部替换",
    close: "关闭",
    outline: "大纲",
    collapse: "收起",
    expand: "展开",
    filterHeadings: "筛选标题...",
    noHeadings: "暂无标题",
    noMatchedHeadings: "无匹配标题",
    recentFiles: "最近文件",
    clear: "清空",
    pinUnpin: "置顶或取消置顶",
    removeItem: "删除项",
    filterRecent: "筛选最近文件...",
    noRecentFiles: "暂无最近文件",
    noMatchedFiles: "无匹配文件",
    retry: "重试",
    commandPalette: "命令面板",
    usage: "使用文档",
    settings: "设置",
    globalSettings: "全局设置",
    settingsTheme: "主题",
    settingsLanguage: "语言",
    settingsEditor: "编辑器",
  settingsFontSize: "字体大小",
    settingsFontFamily: "字体",
    settingsAppearance: "显示",
    settingsLineNumbers: "显示行号",
    settingsWrapLines: "自动换行",
    settingsStatusbar: "显示状态栏",
    settingsSidebar: "显示侧边栏",
    settingsScrollSync: "滚动同步",
    settingsAutosave: "自动保存策略",
    settingsAutosaveEnable: "启用自动保存",
    settingsAutosaveInterval: "自动保存间隔",
    settingsShortcut: "快捷键策略",
    settingsRedoY: "启用 Ctrl/Cmd+Y 重做",
    settingsZenShortcut: "启用 Ctrl/Cmd+Shift+X 专注模式",
    settingsHint: "所有设置会自动保存",
    settingsKeymap: "自定义快捷键",
    keyCommandPalette: "命令面板",
    keyHelp: "快捷键帮助",
    keySettings: "设置页",
    keyUsage: "使用文档",
    keyZen: "专注模式",
    settingsResetShortcuts: "重置快捷键",
    settingsShortcutInvalid: "无效快捷键已恢复默认: {name}",
    settingsShortcutConflict: "冲突: {pattern} -> {names}",
    settingsShortcutConflictRejected: "快捷键冲突，已回退: {name}",
    settingsShortcutNeedModifier: "快捷键至少包含一个修饰键（Ctrl/Cmd/Shift/Alt）",
    settingsShortcutPlaceholder: "在输入框中按下快捷键",
    settingsUseDefault: "默认",
    searchShortcuts: "搜索快捷键...",
    noMatchedShortcuts: "无匹配快捷键",
    shortcutGroupFile: "文件",
    shortcutGroupEdit: "编辑",
    shortcutGroupView: "视图",
    shortcutGroupFormat: "格式",
    shortcutGroupTools: "工具",
    settingsImport: "导入设置",
    settingsExport: "导出设置",
    settingsImportOk: "设置已导入",
    settingsImportFail: "设置导入失败",
    typeCommand: "输入命令...",
    noCommandFound: "未找到命令",
    keyboardShortcuts: "快捷键",
    autosaveErrorDetails: "自动保存错误详情",
    sort: "排序",
    newest: "最新",
    oldest: "最早",
    selectAll: "全选",
    unselectAll: "取消全选",
    deleteSelected: "删除已选 ({count})",
    deleteFiltered: "删除筛选 ({count})",
    searchError: "搜索错误内容...",
    noHistory: "暂无历史",
    noMatchedErrors: "无匹配错误",
    copyError: "复制错误",
    exportLog: "导出日志",
    exportHtml: "导出 HTML",
    resetLayout: "重置布局尺寸",
    clearError: "清空错误",
    undoDelete: "撤销删除 ({count})",
    redoDelete: "重做删除 ({count})",
    confirmDelete: "确认删除",
    delete: "删除",
    cancel: "取消",
    unsavedChanges: "未保存更改",
    unsavedConfirm: "当前文档有未保存修改，是否丢弃并继续？",
    discardContinue: "丢弃并继续",
    langButton: "EN",
    autosaveIntervalTitle: "自动保存间隔",
  },
  en: {
    sidebar: "Sidebar",
    split: "Split",
    edit: "Edit",
    preview: "Preview",
    restore: "Restore",
    maximize: "Maximize",
    new: "New",
    open: "Open",
    save: "Save",
    saveAs: "Save As",
    autosave: "AutoSave",
    auto: "Auto",
    on: "on",
    off: "off",
    utf8: "UTF-8",
    lineCol: "Ln {line}, Col {col}",
    words: "{count} words",
    chars: "{count} chars",
    minRead: "{count} min read",
    autosaveIdle: "Autosave: idle",
    autosavePending: "Autosave: pending",
    autosaveSaving: "Autosave: saving...",
    autosaveFailed: "Autosave: failed",
    autosaveSaved: "Autosave: saved",
    autosaveSavedAt: "Autosave: saved {at}",
    cycleAutosaveInterval: "Cycle Autosave Interval",
    showAutosaveError: "Show Autosave Error",
    copyAutosaveError: "Copy Autosave Error",
    exportAutosaveErrorLog: "Export Autosave Error Log",
    clearAutosaveError: "Clear Autosave Error",
    findReplace: "Find/Replace",
    goToLine: "Go to Line",
    pdf: "PDF",
    shortcuts: "Shortcuts",
    sync: "Sync",
    lineNo: "Ln#",
    wrap: "Wrap",
    bar: "Bar",
    zen: "Zen",
    light: "Light",
    dark: "Dark",
    command: "Command",
    bold: "Bold",
    italic: "Italic",
    quote: "Quote",
    list: "List",
    code: "Code",
    codeBlock: "Code Block",
    editorFontMinus: "Editor Font -",
    editorFontPlus: "Editor Font +",
    editorFontReset: "Editor Font Reset",
    find: "Find",
    replaceWith: "Replace with",
    matchCase: "Match Case",
    findNext: "Find Next",
    replace: "Replace",
    replaceAll: "Replace All",
    close: "Close",
    outline: "Outline",
    collapse: "Collapse",
    expand: "Expand",
    filterHeadings: "Filter headings...",
    noHeadings: "No headings",
    noMatchedHeadings: "No matched headings",
    recentFiles: "Recent Files",
    clear: "Clear",
    pinUnpin: "Pin or unpin item",
    removeItem: "Remove item",
    filterRecent: "Filter recent files...",
    noRecentFiles: "No recent files",
    noMatchedFiles: "No matched files",
    retry: "Retry",
    commandPalette: "Command Palette",
    usage: "Usage",
    settings: "Settings",
    globalSettings: "Global Settings",
    settingsTheme: "Theme",
    settingsLanguage: "Language",
    settingsEditor: "Editor",
    settingsFontSize: "Font Size",
    settingsFontFamily: "Font Family",
    settingsAppearance: "Appearance",
    settingsLineNumbers: "Show line numbers",
    settingsWrapLines: "Wrap lines",
    settingsStatusbar: "Show statusbar",
    settingsSidebar: "Show sidebar",
    settingsScrollSync: "Scroll sync",
    settingsAutosave: "Autosave Strategy",
    settingsAutosaveEnable: "Enable autosave",
    settingsAutosaveInterval: "Autosave interval",
    settingsShortcut: "Shortcut Strategy",
    settingsRedoY: "Enable Ctrl/Cmd+Y for redo",
    settingsZenShortcut: "Enable Ctrl/Cmd+Shift+X for Zen",
    settingsHint: "All settings are saved automatically",
    settingsKeymap: "Custom Shortcuts",
    keyCommandPalette: "Command Palette",
    keyHelp: "Help Panel",
    keySettings: "Settings Panel",
    keyUsage: "Usage Panel",
    keyZen: "Zen Mode",
    settingsResetShortcuts: "Reset Shortcuts",
    settingsShortcutInvalid: "Invalid shortcut reset to default: {name}",
    settingsShortcutConflict: "Conflict: {pattern} -> {names}",
    settingsShortcutConflictRejected: "Shortcut conflict, reverted: {name}",
    settingsShortcutNeedModifier: "Shortcut must include at least one modifier (Ctrl/Cmd/Shift/Alt)",
    settingsShortcutPlaceholder: "Press shortcut in this field",
    settingsUseDefault: "Default",
    searchShortcuts: "Search shortcuts...",
    noMatchedShortcuts: "No matched shortcuts",
    shortcutGroupFile: "File",
    shortcutGroupEdit: "Edit",
    shortcutGroupView: "View",
    shortcutGroupFormat: "Format",
    shortcutGroupTools: "Tools",
    settingsImport: "Import Settings",
    settingsExport: "Export Settings",
    settingsImportOk: "Settings imported",
    settingsImportFail: "Settings import failed",
    typeCommand: "Type a command...",
    noCommandFound: "No command found",
    keyboardShortcuts: "Keyboard Shortcuts",
    autosaveErrorDetails: "Autosave Error Details",
    sort: "Sort",
    newest: "Newest",
    oldest: "Oldest",
    selectAll: "Select All",
    unselectAll: "Unselect All",
    deleteSelected: "Delete Selected ({count})",
    deleteFiltered: "Delete Filtered ({count})",
    searchError: "Search error text...",
    noHistory: "No history",
    noMatchedErrors: "No matched errors",
    copyError: "Copy Error",
    exportLog: "Export Log",
    exportHtml: "Export HTML",
    resetLayout: "Reset Layout Sizes",
    clearError: "Clear Error",
    undoDelete: "Undo Delete ({count})",
    redoDelete: "Redo Delete ({count})",
    confirmDelete: "Confirm Delete",
    delete: "Delete",
    cancel: "Cancel",
    unsavedChanges: "Unsaved Changes",
    unsavedConfirm: "Current document has unsaved edits. Discard and continue?",
    discardContinue: "Discard and Continue",
    langButton: "中文",
    autosaveIntervalTitle: "Autosave interval",
  },
};

const md: MarkdownIt = new MarkdownIt({
  html: true,
  linkify: true,
  typographer: true,
  breaks: true,
  highlight: (str: string, lang: string): string => {
    if (lang && hljs.getLanguage(lang)) {
      return `<pre class=\"hljs\"><code>${hljs.highlight(str, { language: lang }).value}</code></pre>`;
    }
    return `<pre class=\"hljs\"><code>${escapeHtml(str)}</code></pre>`;
  },
});
md.use(markdownItTaskLists, { enabled: true, label: true, labelAfter: true });

const injectSourceLineRule = (tokenName: string): void => {
  const fallback =
    md.renderer.rules[tokenName] ??
    ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));

  md.renderer.rules[tokenName] = (tokens, idx, options, env, self) => {
    const token = tokens[idx];
    if (token.map && token.map.length > 0) {
      token.attrSet("data-source-line", String(token.map[0]));
    }
    return fallback(tokens, idx, options, env, self);
  };
};

[
  "paragraph_open",
  "heading_open",
  "blockquote_open",
  "bullet_list_open",
  "ordered_list_open",
  "list_item_open",
  "table_open",
  "hr",
  "fence",
  "code_block",
].forEach(injectSourceLineRule);

const initialMarkdown = `# nmd

欢迎使用 nmd。

## 第六步：滚动同步、自动保存、最近文件

- 编辑区滚动时预览区跟随
- 预览区滚动时编辑区跟随
- 自动保存（防抖）
- 最近文件列表（点击快速打开）

### 快捷键

- Ctrl/Cmd + H: 打开替换面板
- Ctrl/Cmd + S: 保存
`;

const content = ref(initialMarkdown);
const fileName = ref("untitled.md");
const filePath = ref("");
const dirty = ref(false);
const docTabs = ref<DocTab[]>([
  {
    id: `tab-${Date.now().toString(36)}`,
    content: initialMarkdown,
    name: "untitled.md",
    path: "",
    dirty: false,
  },
]);
const activeTabId = ref(docTabs.value[0].id);
const statusText = ref("Ready");
const showCommandPalette = ref(false);
const showHelpPanel = ref(false);
const showSettingsPanel = ref(false);
const showUsagePanel = ref(false);
const showAutosaveErrorPanel = ref(false);
const showDeleteConfirm = ref(false);
const showUnsavedConfirm = ref(false);
const isDarkTheme = ref(false);
const uiLanguage = ref<UILanguage>("zh");
const isZenMode = ref(false);
const isScrollSyncEnabled = ref(true);
const showLineNumbers = ref(true);
const wrapLines = ref(true);
const editorFontSize = ref(14);
const editorFontFamily = ref<string>(EDITOR_FONT_FAMILY_OPTIONS[0].value);
const showStatusbar = ref(true);
const isAutosaveEnabled = ref(true);
const autosaveIntervalMs = ref<number>(AUTOSAVE_INTERVAL_OPTIONS[0]);
const showReplacePanel = ref(false);
const showSidebar = ref(false);
const viewMode = ref<"split" | "edit" | "preview">("split");
const searchQuery = ref("");
const replaceQuery = ref("");
const helpQuery = ref("");
const matchCase = ref(false);
const recentFiles = ref<RecentFile[]>([]);
const pinnedRecentPaths = ref<string[]>([]);
const outlineQuery = ref("");
const recentQuery = ref("");
const draggingRecentPath = ref("");
const cursorLine = ref(1);
const cursorCol = ref(1);
const cursorPos = ref(0);
const paletteQuery = ref("");
const paletteActiveIndex = ref(0);
const helpActiveIndex = ref(0);
const imagePreviewMap = ref<Record<string, string>>({});
const imageStatus = ref("Image: idle");
const autosaveState = ref<"idle" | "pending" | "saving" | "saved" | "error">("idle");
const autosaveErrorText = ref("");
const autosaveErrorHistory = ref<AutosaveErrorEntry[]>([]);
const autosaveErrorActiveId = ref("");
const autosaveErrorSourceFilter = ref<AutosaveErrorSourceFilter>("all");
const autosaveErrorQuery = ref("");
const autosaveErrorSortOrder = ref<AutosaveErrorSortOrder>("desc");
const autosaveErrorSelectedIds = ref<string[]>([]);
const autosaveErrorUndoStack = ref<AutosaveErrorEntry[][]>([]);
const autosaveErrorRedoStack = ref<AutosaveErrorEntry[][]>([]);
const deleteConfirmText = ref("");
const enableRedoWithY = ref(true);
const enableZenShortcut = ref(true);
const shortcutBindings = ref<Record<ShortcutBindingKey, string>>({ ...DEFAULT_SHORTCUT_BINDINGS });
const shortcutBindingsCommitted = ref<Record<ShortcutBindingKey, string>>({ ...DEFAULT_SHORTCUT_BINDINGS });
const autosaveAt = ref("");
const isWindowMaximised = ref(false);
const splitRatio = ref(DEFAULT_SPLIT_RATIO);
const sidebarWidth = ref(DEFAULT_SIDEBAR_WIDTH);
const windowWidth = ref(1280);
const collapsedOutlineMap = ref<Record<string, boolean>>({});
const helpCollapsedGroups = ref<Record<string, boolean>>({});
const previewActiveLine = ref(-1);

const editorRoot = ref<HTMLDivElement | null>(null);
const previewRef = ref<HTMLDivElement | null>(null);
const outlineListRef = ref<HTMLDivElement | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const settingsImportInput = ref<HTMLInputElement | null>(null);
const searchInput = ref<HTMLInputElement | null>(null);
const docZoneRef = ref<HTMLDivElement | null>(null);
const workspaceRef = ref<HTMLDivElement | null>(null);

const t = (key: string): string => I18N[uiLanguage.value][key] ?? key;
const tf = (key: string, vars: Record<string, string | number>): string => {
  return t(key).replace(/\{(\w+)\}/g, (_m, name: string) => String(vars[name] ?? ""));
};

let editorView: EditorView | null = null;
let editorScrollElement: HTMLElement | null = null;
let autosaveTimer: ReturnType<typeof setTimeout> | null = null;
let syncingFromEditor = false;
let syncingFromPreview = false;
let ignoreEditorScrollUntil = 0;
let ignorePreviewScrollUntil = 0;
let pendingUnsavedAction: (() => void | Promise<void>) | null = null;
let pendingDeleteAction: (() => void) | null = null;
const loadingImageSources = new Set<string>();
const onWindowFocus = (): void => {
  void refreshWindowMaximisedState();
};
const onWindowResize = (): void => {
  windowWidth.value = window.innerWidth;
};
const onBeforeUnload = (event: BeforeUnloadEvent): void => {
  if (!hasDirtyTabs.value) return;
  event.preventDefault();
  event.returnValue = "";
};

const getTabIndexById = (id: string): number => docTabs.value.findIndex((tab) => tab.id === id);

const syncActiveTabFromState = (): void => {
  const idx = getTabIndexById(activeTabId.value);
  if (idx < 0) return;
  docTabs.value[idx] = {
    ...docTabs.value[idx],
    content: content.value,
    name: fileName.value,
    path: filePath.value,
    dirty: dirty.value,
  };
};

const activateTab = (id: string): void => {
  if (id === activeTabId.value) return;
  syncActiveTabFromState();
  const idx = getTabIndexById(id);
  if (idx < 0) return;
  const tab = docTabs.value[idx];
  activeTabId.value = tab.id;
  setDocument(tab.content, tab.name, tab.path, tab.dirty);
  updateStatus(`Switched: ${tab.name}`);
};

const openInNewTab = (nextContent: string, nextName: string, nextPath: string, nextDirty = false): void => {
  syncActiveTabFromState();
  const tab: DocTab = {
    id: `tab-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
    content: nextContent,
    name: nextName,
    path: nextPath,
    dirty: nextDirty,
  };
  docTabs.value = [...docTabs.value, tab];
  activeTabId.value = tab.id;
  setDocument(tab.content, tab.name, tab.path, tab.dirty);
};

const closeTabNow = (id: string): void => {
  if (docTabs.value.length <= 1) {
    setDocument("# New Document\n\n", "untitled.md", "", false);
    syncActiveTabFromState();
    updateStatus("Tab reset");
    return;
  }
  const idx = getTabIndexById(id);
  if (idx < 0) return;
  const nextTabs = docTabs.value.filter((tab) => tab.id !== id);
  docTabs.value = nextTabs;
  if (activeTabId.value === id) {
    const nextIdx = Math.max(0, Math.min(idx, nextTabs.length - 1));
    const nextTab = nextTabs[nextIdx];
    activeTabId.value = nextTab.id;
    setDocument(nextTab.content, nextTab.name, nextTab.path, nextTab.dirty);
  }
  updateStatus("Tab closed");
};

const requestCloseTab = (id: string): void => {
  const idx = getTabIndexById(id);
  if (idx < 0) return;
  const tab = docTabs.value[idx];
  if (tab.id === activeTabId.value && tab.dirty) {
    pendingUnsavedAction = () => closeTabNow(id);
    showUnsavedConfirm.value = true;
    updateStatus("Unsaved changes detected");
    return;
  }
  if (tab.dirty && !window.confirm(`Close unsaved tab "${tab.name}"?`)) return;
  closeTabNow(id);
};

const themeCompartment = new Compartment();
const lineNumberCompartment = new Compartment();
const wrapCompartment = new Compartment();
const fontSizeCompartment = new Compartment();
const fontFamilyCompartment = new Compartment();

const editorThemeLight = EditorView.theme({
  "&": {
    height: "100%",
    color: "#0f172a",
    backgroundColor: "#ffffff",
  },
  ".cm-scroller": { overflow: "auto" },
  ".cm-content": { padding: "16px", lineHeight: "1.65" },
  ".cm-gutters": {
    backgroundColor: "#f8fbff",
    color: "#6b7280",
    borderRight: "1px solid #e6ebf2",
  },
  ".cm-activeLine": { backgroundColor: "#f8fafc" },
});

const editorThemeDark = EditorView.theme({
  "&": { backgroundColor: "#111827" },
  ".cm-gutters": {
    backgroundColor: "#111827",
    color: "#9ca3af",
    borderRight: "1px solid #1f2937",
  },
});

const currentEditorTheme = () => (isDarkTheme.value ? [oneDark, editorThemeDark] : [editorThemeLight]);
const editorFontSizeTheme = (size: number) =>
  EditorView.theme({
    "&": {
      fontSize: `${size}px`,
    },
  });

const editorFontFamilyTheme = (family: string) =>
  EditorView.theme({
    "&": {
      fontFamily: family,
    },
    ".cm-content, .cm-gutters": {
      fontFamily: family,
    },
  });

const toPreviewImageSrc = (src: string): string => {
  const s = src.trim();
  if (!s) return s;
  if (/^(https?:|data:|file:)/i.test(s)) return s;

  if (s.startsWith("/")) {
    return `file://${encodeURI(s)}`;
  }

  if (filePath.value) {
    const normalizedDocPath = filePath.value.replace(/\\/g, "/");
    const slash = normalizedDocPath.lastIndexOf("/");
    const baseDir = slash >= 0 ? normalizedDocPath.slice(0, slash + 1) : normalizedDocPath;
    try {
      return new URL(s, `file://${encodeURI(baseDir)}`).toString();
    } catch {
      return s;
    }
  }
  return s;
};

const lookupImagePreviewMap = (src: string): string | null => {
  if (imagePreviewMap.value[src]) return imagePreviewMap.value[src];
  try {
    const decoded = decodeURI(src);
    if (imagePreviewMap.value[decoded]) return imagePreviewMap.value[decoded];
  } catch {
    // ignore decode failure
  }
  const encoded = encodeURI(src);
  if (imagePreviewMap.value[encoded]) return imagePreviewMap.value[encoded];
  return null;
};

const rewriteImageSrcInHTML = (html: string): string => {
  if (typeof window === "undefined" || typeof DOMParser === "undefined") {
    return html;
  }
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");
  const imgs = doc.querySelectorAll("img");
  imgs.forEach((img) => {
    const src = img.getAttribute("src") || "";
    if (!src) return;
    const mapped = lookupImagePreviewMap(src);
    img.setAttribute("src", mapped || toPreviewImageSrc(src));
  });
  return doc.body.innerHTML;
};

const renderedHtml = computed(() => rewriteImageSrcInHTML(md.render(content.value)));
const usageHtml = computed(() => rewriteImageSrcInHTML(md.render(usageMarkdown)));
const plainText = computed(() =>
  content.value
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`[^`]*`/g, " ")
    .replace(/!\[[^\]]*\]\([^)]+\)/g, " ")
    .replace(/\[[^\]]+\]\([^)]+\)/g, " ")
    .replace(/[#>*_\-\[\]\(\)]/g, " ")
    .replace(/\s+/g, " ")
    .trim(),
);
const wordCount = computed(() => {
  const englishWords = plainText.value.match(/[A-Za-z0-9]+(?:'[A-Za-z0-9]+)*/g)?.length ?? 0;
  const cjkChars = plainText.value.match(/[\u3400-\u9FFF]/g)?.length ?? 0;
  return englishWords + cjkChars;
});
const charCount = computed(() => plainText.value.replace(/\s/g, "").length);
const readingMinutes = computed(() => {
  // Rough reading speed blend for mixed language text.
  const minutes = wordCount.value / 220;
  return Math.max(1, Math.round(minutes));
});
const autosaveLabel = computed(() => {
  if (!isAutosaveEnabled.value) return `${t("autosave")}: ${t("off")}`;
  if (autosaveState.value === "idle") return t("autosaveIdle");
  if (autosaveState.value === "pending") return t("autosavePending");
  if (autosaveState.value === "saving") return t("autosaveSaving");
  if (autosaveState.value === "error") return t("autosaveFailed");
  if (autosaveAt.value) return tf("autosaveSavedAt", { at: autosaveAt.value });
  return t("autosaveSaved");
});
const autosaveIntervalLabel = computed(() => `${Math.round(autosaveIntervalMs.value / 100) / 10}s`);
const autosaveErrorLabel = computed(() => {
  if (!autosaveErrorText.value) return "";
  const maxLen = 68;
  return autosaveErrorText.value.length > maxLen ? `${autosaveErrorText.value.slice(0, maxLen)}...` : autosaveErrorText.value;
});
const activeAutosaveError = computed<AutosaveErrorEntry | null>(() => {
  if (filteredAutosaveErrorHistory.value.length === 0) return null;
  const hit = filteredAutosaveErrorHistory.value.find((item) => item.id === autosaveErrorActiveId.value);
  return hit || filteredAutosaveErrorHistory.value[0];
});
const activeAutosaveErrorDetail = computed(() => {
  const active = activeAutosaveError.value;
  if (active) {
    return `[${active.at}] ${active.source}\n${active.message}`;
  }
  if (autosaveErrorHistory.value.length > 0 && filteredAutosaveErrorHistory.value.length === 0) {
    return "No matched error under current source filter.";
  }
  if (autosaveErrorText.value) return autosaveErrorText.value;
  return "No autosave error";
});
const autosaveIntervalOptions = AUTOSAVE_INTERVAL_OPTIONS.map((ms) => ({
  value: ms,
  label: `${Math.round(ms / 100) / 10}s`,
}));
const editorFontFamilyOptions = EDITOR_FONT_FAMILY_OPTIONS;
const autosaveErrorSourceOptions = computed<Array<{ value: AutosaveErrorSourceFilter; label: string }>>(() => [
  { value: "all", label: uiLanguage.value === "zh" ? "全部" : "All" },
  { value: "autosave", label: uiLanguage.value === "zh" ? "自动保存" : "Autosave" },
  { value: "save", label: uiLanguage.value === "zh" ? "保存" : "Save" },
  { value: "saveAs", label: uiLanguage.value === "zh" ? "另存为" : "SaveAs" },
]);
const filteredAutosaveErrorHistory = computed(() => {
  const sorted = autosaveErrorSortOrder.value === "asc" ? [...autosaveErrorHistory.value].reverse() : autosaveErrorHistory.value;
  const q = autosaveErrorQuery.value.trim().toLowerCase();
  return sorted.filter((item) => {
    const sourceMatch = autosaveErrorSourceFilter.value === "all" || item.source === autosaveErrorSourceFilter.value;
    if (!sourceMatch) return false;
    if (!q) return true;
    return item.message.toLowerCase().includes(q) || item.at.toLowerCase().includes(q) || item.source.toLowerCase().includes(q);
  });
});
const autosaveSortLabel = computed(() => (autosaveErrorSortOrder.value === "desc" ? t("newest") : t("oldest")));
const selectedAutosaveErrorCount = computed(() => autosaveErrorSelectedIds.value.length);
const undoDeletedAutosaveErrorCount = computed(() => autosaveErrorUndoStack.value[0]?.length ?? 0);
const redoDeletedAutosaveErrorCount = computed(() => autosaveErrorRedoStack.value[0]?.length ?? 0);
const allFilteredAutosaveErrorsSelected = computed(() => {
  if (filteredAutosaveErrorHistory.value.length === 0) return false;
  const selected = new Set(autosaveErrorSelectedIds.value);
  return filteredAutosaveErrorHistory.value.every((item) => selected.has(item.id));
});
const fileLabel = computed(() => (dirty.value ? `${fileName.value} *` : fileName.value));
const filePathLabel = computed(() => (filePath.value ? filePath.value : "unsaved"));
const windowTitle = computed(() => `nmd - ${fileLabel.value}`);
const hasDirtyTabs = computed(() => docTabs.value.some((tab) => tab.dirty));
const docZoneStyle = computed(() => {
  if (viewMode.value !== "split") return {};
  const left = Math.max(20, Math.min(80, splitRatio.value));
  const right = 100 - left;
  return {
    gridTemplateColumns: `minmax(320px, ${left}fr) 8px minmax(320px, ${right}fr)`,
  };
});
const showSidebarSplitter = computed(() => showSidebar.value && !isZenMode.value && windowWidth.value > 1200);
const workspaceStyle = computed(() => {
  if (!showSidebarSplitter.value) return {};
  const width = Math.max(180, Math.min(420, sidebarWidth.value));
  return {
    gridTemplateColumns: `${width}px 8px minmax(0, 1fr)`,
  };
});
const layoutStyle = computed(() => ({
  "--nmd-editor-font": editorFontFamily.value,
}));
const normalizeShortcutToken = (token: string): string => {
  const s = token.trim().toLowerCase();
  if (s === "slash") return "/";
  if (s === "comma") return ",";
  if (s === "period" || s === "dot") return ".";
  if (s === "minus" || s === "dash" || s === "_" || s === "-") return "-";
  if (s === "equal" || s === "equals" || s === "plus" || s === "=" || s === "+") return "=";
  if (s === "backtick" || s === "grave") return "`";
  if (s === "question") return "/";
  if (s === "space") return " ";
  return s;
};

const parseShortcutPattern = (pattern: string): ShortcutPattern | null => {
  const compact = pattern.replace(/\s+/g, "");
  if (!compact) return null;
  const parts = compact.split("+").filter(Boolean);
  if (parts.length === 0) return null;
  let primary = false;
  let shift = false;
  let alt = false;
  let key = "";
  for (const part of parts) {
    const lower = part.toLowerCase();
    if (
      lower === "ctrl" ||
      lower === "control" ||
      lower === "cmd" ||
      lower === "command" ||
      lower === "meta" ||
      lower === "primary" ||
      lower === "ctrl/cmd" ||
      lower === "cmd/ctrl"
    ) {
      primary = true;
      continue;
    }
    if (lower === "shift") {
      shift = true;
      continue;
    }
    if (lower === "alt" || lower === "option") {
      alt = true;
      continue;
    }
    key = normalizeShortcutToken(part);
  }
  if (!key) return null;
  return { primary, shift, alt, key };
};

const formatShortcutPattern = (pattern: ShortcutPattern): string => {
  const parts: string[] = [];
  if (pattern.primary) parts.push("Ctrl/Cmd");
  if (pattern.alt) parts.push("Alt");
  if (pattern.shift) parts.push("Shift");
  const keyDisplay = pattern.key === " " ? "Space" : pattern.key.length === 1 ? pattern.key.toUpperCase() : pattern.key;
  parts.push(keyDisplay);
  return parts.join("+");
};

const normalizeShortcutBinding = (value: string, fallback: string): string => {
  const parsed = parseShortcutPattern(value) ?? parseShortcutPattern(fallback);
  if (!parsed) return fallback;
  return formatShortcutPattern(parsed);
};

const normalizeShortcutBindings = (raw: unknown): Record<ShortcutBindingKey, string> => {
  const source = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return {
    commandPalette: normalizeShortcutBinding(
      typeof source.commandPalette === "string" ? source.commandPalette : DEFAULT_SHORTCUT_BINDINGS.commandPalette,
      DEFAULT_SHORTCUT_BINDINGS.commandPalette,
    ),
    help: normalizeShortcutBinding(
      typeof source.help === "string" ? source.help : DEFAULT_SHORTCUT_BINDINGS.help,
      DEFAULT_SHORTCUT_BINDINGS.help,
    ),
    settings: normalizeShortcutBinding(
      typeof source.settings === "string" ? source.settings : DEFAULT_SHORTCUT_BINDINGS.settings,
      DEFAULT_SHORTCUT_BINDINGS.settings,
    ),
    usage: normalizeShortcutBinding(
      typeof source.usage === "string" ? source.usage : DEFAULT_SHORTCUT_BINDINGS.usage,
      DEFAULT_SHORTCUT_BINDINGS.usage,
    ),
    zen: normalizeShortcutBinding(
      typeof source.zen === "string" ? source.zen : DEFAULT_SHORTCUT_BINDINGS.zen,
      DEFAULT_SHORTCUT_BINDINGS.zen,
    ),
  };
};

const normalizeShortcutEventKey = (raw: string): string => {
  const s = raw.toLowerCase();
  if (s === "?") return "/";
  if (s === ">" || s === ".") return ".";
  if (s === "<" || s === ",") return ",";
  if (s === "_" || s === "-") return "-";
  if (s === "+" || s === "=") return "=";
  return normalizeShortcutToken(s);
};

const getShortcutBindingLabel = (key: ShortcutBindingKey): string => {
  if (key === "commandPalette") return t("keyCommandPalette");
  if (key === "help") return t("keyHelp");
  if (key === "settings") return t("keySettings");
  if (key === "usage") return t("keyUsage");
  return t("keyZen");
};

const listShortcutConflicts = (
  bindings: Record<ShortcutBindingKey, string>,
  zenEnabled: boolean,
): Array<{ pattern: string; keys: ShortcutBindingKey[] }> => {
  const map = new Map<string, ShortcutBindingKey[]>();
  (Object.keys(bindings) as ShortcutBindingKey[]).forEach((key) => {
    if (key === "zen" && !zenEnabled) return;
    const parsed = parseShortcutPattern(bindings[key]);
    if (!parsed) return;
    const normalized = formatShortcutPattern(parsed);
    const current = map.get(normalized) || [];
    current.push(key);
    map.set(normalized, current);
  });
  const list: Array<{ pattern: string; keys: ShortcutBindingKey[] }> = [];
  for (const [pattern, keys] of map.entries()) {
    if (keys.length > 1) list.push({ pattern, keys });
  }
  return list;
};

const invalidShortcutKeys = computed<Set<ShortcutBindingKey>>(() => {
  const invalid = new Set<ShortcutBindingKey>();
  (Object.keys(shortcutBindings.value) as ShortcutBindingKey[]).forEach((key) => {
    if (!parseShortcutPattern(shortcutBindings.value[key])) invalid.add(key);
  });
  return invalid;
});

const shortcutConflicts = computed<Array<{ pattern: string; keys: ShortcutBindingKey[] }>>(() =>
  listShortcutConflicts(shortcutBindings.value, enableZenShortcut.value),
);

const matchesShortcutBinding = (event: KeyboardEvent, key: ShortcutBindingKey): boolean => {
  const parsed = parseShortcutPattern(shortcutBindings.value[key]);
  if (!parsed) return false;
  const isPrimary = event.metaKey || event.ctrlKey;
  if (parsed.primary !== isPrimary) return false;
  if (parsed.shift !== event.shiftKey) return false;
  if (parsed.alt !== event.altKey) return false;
  const pressedKey = normalizeShortcutEventKey(event.key);
  return pressedKey === parsed.key;
};

const normalizeShortcutInput = (key: ShortcutBindingKey): void => {
  const parsed = parseShortcutPattern(shortcutBindings.value[key]);
  if (!parsed) {
    shortcutBindings.value[key] = DEFAULT_SHORTCUT_BINDINGS[key];
    updateStatus(tf("settingsShortcutInvalid", { name: getShortcutBindingLabel(key) }));
    return;
  }
  const normalized = formatShortcutPattern(parsed);
  const nextBindings = { ...shortcutBindings.value, [key]: normalized };
  const conflict = listShortcutConflicts(nextBindings, enableZenShortcut.value).find((item) => item.keys.includes(key));
  if (conflict) {
    shortcutBindings.value[key] = shortcutBindingsCommitted.value[key];
    updateStatus(tf("settingsShortcutConflictRejected", { name: getShortcutBindingLabel(key) }));
    return;
  }
  shortcutBindings.value[key] = normalized;
  shortcutBindingsCommitted.value = { ...nextBindings };
};

const resetShortcutBindings = (): void => {
  shortcutBindings.value = { ...DEFAULT_SHORTCUT_BINDINGS };
  shortcutBindingsCommitted.value = { ...DEFAULT_SHORTCUT_BINDINGS };
};

const resetShortcutBinding = (key: ShortcutBindingKey): void => {
  const next = DEFAULT_SHORTCUT_BINDINGS[key];
  shortcutBindings.value[key] = next;
  shortcutBindingsCommitted.value = { ...shortcutBindingsCommitted.value, [key]: next };
};

const exportSettings = (): void => {
  const payload: ExportedSettings = {
    version: 1,
    theme: isDarkTheme.value ? "dark" : "light",
    language: uiLanguage.value,
    showSidebar: showSidebar.value,
    showStatusbar: showStatusbar.value,
    showLineNumbers: showLineNumbers.value,
    wrapLines: wrapLines.value,
    scrollSync: isScrollSyncEnabled.value,
    editorFontSize: editorFontSize.value,
    editorFontFamily: editorFontFamily.value,
    autosaveEnabled: isAutosaveEnabled.value,
    autosaveIntervalMs: autosaveIntervalMs.value,
    enableRedoWithY: enableRedoWithY.value,
    enableZenShortcut: enableZenShortcut.value,
    shortcutBindings: { ...shortcutBindings.value },
  };
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "nmd-settings.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
};

const applyImportedSettings = (payload: ExportedSettings): void => {
  isDarkTheme.value = payload.theme === "dark";
  uiLanguage.value = payload.language === "en" ? "en" : "zh";
  showSidebar.value = Boolean(payload.showSidebar);
  showStatusbar.value = Boolean(payload.showStatusbar);
  showLineNumbers.value = Boolean(payload.showLineNumbers);
  wrapLines.value = Boolean(payload.wrapLines);
  isScrollSyncEnabled.value = Boolean(payload.scrollSync);
  editorFontSize.value = Math.max(12, Math.min(22, Number(payload.editorFontSize) || 14));
  const importedFamily = String(payload.editorFontFamily || "").trim();
  const validFamily = EDITOR_FONT_FAMILY_OPTIONS.some((item) => item.value === importedFamily);
  editorFontFamily.value = validFamily ? importedFamily : EDITOR_FONT_FAMILY_OPTIONS[0].value;
  isAutosaveEnabled.value = Boolean(payload.autosaveEnabled);
  if (
    AUTOSAVE_INTERVAL_OPTIONS.includes(
      Number(payload.autosaveIntervalMs) as (typeof AUTOSAVE_INTERVAL_OPTIONS)[number],
    )
  ) {
    autosaveIntervalMs.value = Number(payload.autosaveIntervalMs);
  }
  enableRedoWithY.value = Boolean(payload.enableRedoWithY);
  enableZenShortcut.value = Boolean(payload.enableZenShortcut);
  shortcutBindings.value = normalizeShortcutBindings(payload.shortcutBindings);
  shortcutBindingsCommitted.value = { ...shortcutBindings.value };
};

const triggerImportSettings = (): void => {
  settingsImportInput.value?.click();
};

const onImportSettingsSelected = async (event: Event): Promise<void> => {
  const input = event.target as HTMLInputElement | null;
  const file = input?.files?.[0];
  if (!file) return;
  try {
    const text = await file.text();
    const parsed = JSON.parse(text) as Partial<ExportedSettings>;
    if (parsed.version !== 1) throw new Error("unsupported settings version");
    applyImportedSettings(parsed as ExportedSettings);
    updateStatus(t("settingsImportOk"));
  } catch (error) {
    updateStatus(`${t("settingsImportFail")}: ${String(error)}`);
  } finally {
    if (input) input.value = "";
  }
};

const captureShortcutByEvent = (key: ShortcutBindingKey, event: KeyboardEvent): void => {
  event.preventDefault();
  event.stopPropagation();
  if (event.key === "Tab") return;
  if (event.key === "Escape" && !event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey) return;
  const lowered = event.key.toLowerCase();
  if (lowered === "meta" || lowered === "control" || lowered === "alt" || lowered === "shift") return;
  const hasModifier = event.metaKey || event.ctrlKey || event.altKey || event.shiftKey;
  if (!hasModifier) {
    updateStatus(t("settingsShortcutNeedModifier"));
    return;
  }
  const pattern: ShortcutPattern = {
    primary: event.metaKey || event.ctrlKey,
    shift: event.shiftKey,
    alt: event.altKey,
    key: normalizeShortcutEventKey(event.key),
  };
  shortcutBindings.value[key] = formatShortcutPattern(pattern);
  normalizeShortcutInput(key);
};

type HelpShortcutItem = { id: Command; label: string; shortcut: string };
type HelpShortcutGroup = { id: string; title: string; items: HelpShortcutItem[] };

const helpShortcutGroups = computed<HelpShortcutGroup[]>(() => [
  {
    id: "file",
    title: t("shortcutGroupFile"),
    items: [
      { id: "new", label: t("new"), shortcut: "Ctrl/Cmd+N" },
      { id: "open", label: t("open"), shortcut: "Ctrl/Cmd+O" },
      { id: "save", label: t("save"), shortcut: "Ctrl/Cmd+S" },
      { id: "saveAs", label: t("saveAs"), shortcut: "Ctrl/Cmd+Shift+S" },
      { id: "exportPdf", label: t("pdf"), shortcut: "-" },
      { id: "showUsage", label: t("usage"), shortcut: shortcutBindings.value.usage },
    ],
  },
  {
    id: "edit",
    title: t("shortcutGroupEdit"),
    items: [
      { id: "find", label: t("findNext"), shortcut: "Ctrl/Cmd+F" },
      { id: "replace", label: t("findReplace"), shortcut: "Ctrl/Cmd+H" },
      { id: "gotoLine", label: t("goToLine"), shortcut: "Ctrl/Cmd+L" },
      { id: "undo", label: "Undo", shortcut: "Ctrl/Cmd+Z" },
      { id: "redo", label: "Redo", shortcut: enableRedoWithY.value ? "Ctrl/Cmd+Y / Ctrl/Cmd+Shift+Z" : "Ctrl/Cmd+Shift+Z" },
    ],
  },
  {
    id: "view",
    title: t("shortcutGroupView"),
    items: [
      { id: "viewSplit", label: t("split"), shortcut: "Ctrl/Cmd+1" },
      { id: "viewEditOnly", label: t("edit"), shortcut: "Ctrl/Cmd+2" },
      { id: "viewPreviewOnly", label: t("preview"), shortcut: "Ctrl/Cmd+3" },
      { id: "toggleSidebar", label: t("sidebar"), shortcut: "Ctrl/Cmd+B" },
      { id: "toggleScrollSync", label: t("sync"), shortcut: "Ctrl/Cmd+Shift+Y" },
      { id: "toggleMaximise", label: `${t("maximize")}/${t("restore")}`, shortcut: "Ctrl/Cmd+Shift+M" },
      { id: "toggleZen", label: t("zen"), shortcut: enableZenShortcut.value ? shortcutBindings.value.zen : "-" },
    ],
  },
  {
    id: "format",
    title: t("shortcutGroupFormat"),
    items: [
      { id: "fmtBold", label: t("bold"), shortcut: "Ctrl/Cmd+Shift+B" },
      { id: "fmtItalic", label: t("italic"), shortcut: "Ctrl/Cmd+Shift+I" },
      { id: "fmtCode", label: t("codeBlock"), shortcut: "Ctrl/Cmd+Shift+`" },
      { id: "fmtH1", label: "H1", shortcut: "Ctrl/Cmd+Shift+1" },
      { id: "fmtH2", label: "H2", shortcut: "Ctrl/Cmd+Shift+2" },
      { id: "fmtQuote", label: t("quote"), shortcut: "Ctrl/Cmd+Shift+." },
      { id: "fmtBullet", label: t("list"), shortcut: "Ctrl/Cmd+Shift+8" },
    ],
  },
  {
    id: "tools",
    title: t("shortcutGroupTools"),
    items: [
      { id: "palette", label: t("commandPalette"), shortcut: shortcutBindings.value.commandPalette },
      { id: "settings", label: t("settings"), shortcut: shortcutBindings.value.settings },
      { id: "help", label: t("shortcuts"), shortcut: shortcutBindings.value.help },
      { id: "toggleAutosave", label: t("autosave"), shortcut: "Ctrl/Cmd+Shift+A" },
      { id: "cycleAutosaveInterval", label: t("cycleAutosaveInterval"), shortcut: "Ctrl/Cmd+Shift+T" },
      { id: "showAutosaveError", label: t("showAutosaveError"), shortcut: "Ctrl/Cmd+Shift+E" },
      { id: "copyAutosaveError", label: t("copyAutosaveError"), shortcut: "Ctrl/Cmd+Shift+C" },
      { id: "toggleLanguage", label: t("langButton"), shortcut: "-" },
    ],
  },
]);

const filteredHelpShortcutGroups = computed<HelpShortcutGroup[]>(() => {
  const q = helpQuery.value.trim().toLowerCase();
  if (!q) return helpShortcutGroups.value;
  return helpShortcutGroups.value
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => {
        return (
          group.title.toLowerCase().includes(q) ||
          item.label.toLowerCase().includes(q) ||
          item.shortcut.toLowerCase().includes(q)
        );
      }),
    }))
    .filter((group) => group.items.length > 0);
});

const visibleHelpShortcutItems = computed<Array<{ groupId: string; item: HelpShortcutItem }>>(() => {
  const visible: Array<{ groupId: string; item: HelpShortcutItem }> = [];
  filteredHelpShortcutGroups.value.forEach((group) => {
    if (helpCollapsedGroups.value[group.id]) return;
    group.items.forEach((item) => {
      visible.push({ groupId: group.id, item });
    });
  });
  return visible;
});

const toggleHelpGroup = (id: string): void => {
  const current = Boolean(helpCollapsedGroups.value[id]);
  helpCollapsedGroups.value = { ...helpCollapsedGroups.value, [id]: !current };
};

const paletteCommands = computed<{ id: Command; label: string; shortcut: string }[]>(() => [
  { id: "new", label: t("new"), shortcut: "Ctrl/Cmd+N" },
  { id: "open", label: t("open"), shortcut: "Ctrl/Cmd+O" },
  { id: "save", label: t("save"), shortcut: "Ctrl/Cmd+S" },
  { id: "saveAs", label: t("saveAs"), shortcut: "Ctrl/Cmd+Shift+S" },
  { id: "replace", label: t("findReplace"), shortcut: "Ctrl/Cmd+H" },
  { id: "gotoLine", label: t("goToLine"), shortcut: "Ctrl/Cmd+L" },
  { id: "palette", label: t("commandPalette"), shortcut: shortcutBindings.value.commandPalette },
  { id: "find", label: t("findNext"), shortcut: "Ctrl/Cmd+F" },
  { id: "replaceAll", label: t("replaceAll"), shortcut: "-" },
  { id: "fmtBold", label: t("bold"), shortcut: "Ctrl/Cmd+Shift+B" },
  { id: "fmtItalic", label: t("italic"), shortcut: "Ctrl/Cmd+Shift+I" },
  { id: "fmtCode", label: t("codeBlock"), shortcut: "Ctrl/Cmd+Shift+`" },
  { id: "fmtH1", label: "H1", shortcut: "Ctrl/Cmd+Shift+1" },
  { id: "fmtH2", label: "H2", shortcut: "Ctrl/Cmd+Shift+2" },
  { id: "fmtQuote", label: t("quote"), shortcut: "Ctrl/Cmd+Shift+." },
  { id: "fmtBullet", label: t("list"), shortcut: "Ctrl/Cmd+Shift+8" },
  { id: "export", label: t("exportHtml"), shortcut: "-" },
  { id: "exportPdf", label: t("pdf"), shortcut: "-" },
  { id: "toggleSidebar", label: t("sidebar"), shortcut: "Ctrl/Cmd+B" },
  { id: "toggleZen", label: t("zen"), shortcut: enableZenShortcut.value ? shortcutBindings.value.zen : "-" },
  { id: "toggleScrollSync", label: t("sync"), shortcut: "Ctrl/Cmd+Shift+Y" },
  { id: "toggleLineNumbers", label: t("lineNo"), shortcut: "Ctrl/Cmd+Shift+G" },
  { id: "toggleWrapLines", label: t("wrap"), shortcut: "Ctrl/Cmd+Shift+W" },
  { id: "toggleStatusbar", label: t("bar"), shortcut: "Ctrl/Cmd+Shift+U" },
  { id: "toggleAutosave", label: t("autosave"), shortcut: "Ctrl/Cmd+Shift+A" },
  { id: "cycleAutosaveInterval", label: t("cycleAutosaveInterval"), shortcut: "Ctrl/Cmd+Shift+T" },
  { id: "retryAutosave", label: t("retry"), shortcut: "-" },
  {
    id: "showAutosaveError",
    label: autosaveErrorText.value ? `${t("showAutosaveError")}: ${autosaveErrorLabel.value}` : t("showAutosaveError"),
    shortcut: "Ctrl/Cmd+Shift+E",
  },
  { id: "copyAutosaveError", label: t("copyAutosaveError"), shortcut: "Ctrl/Cmd+Shift+C" },
  { id: "exportAutosaveErrorLog", label: t("exportAutosaveErrorLog"), shortcut: "-" },
  { id: "clearAutosaveError", label: t("clearAutosaveError"), shortcut: "-" },
  { id: "undoAutosaveErrorDelete", label: tf("undoDelete", { count: undoDeletedAutosaveErrorCount.value }), shortcut: "Ctrl/Cmd+Shift+R" },
  { id: "redoAutosaveErrorDelete", label: tf("redoDelete", { count: redoDeletedAutosaveErrorCount.value }), shortcut: "Ctrl/Cmd+Shift+J" },
  { id: "fontSmaller", label: t("editorFontMinus"), shortcut: "Ctrl/Cmd+-" },
  { id: "fontLarger", label: t("editorFontPlus"), shortcut: "Ctrl/Cmd+=" },
  { id: "fontReset", label: t("editorFontReset"), shortcut: "-" },
  { id: "viewSplit", label: t("split"), shortcut: "Ctrl/Cmd+1" },
  { id: "viewEditOnly", label: t("edit"), shortcut: "Ctrl/Cmd+2" },
  { id: "viewPreviewOnly", label: t("preview"), shortcut: "Ctrl/Cmd+3" },
  { id: "resetLayout", label: t("resetLayout"), shortcut: "Ctrl/Cmd+0" },
  { id: "toggleMaximise", label: t("maximize"), shortcut: "Ctrl/Cmd+Shift+M" },
  { id: "help", label: t("shortcuts"), shortcut: shortcutBindings.value.help },
  { id: "settings", label: t("settings"), shortcut: shortcutBindings.value.settings },
  { id: "showUsage", label: t("usage"), shortcut: shortcutBindings.value.usage },
  { id: "toggleTheme", label: isDarkTheme.value ? t("light") : t("dark"), shortcut: "-" },
  { id: "toggleLanguage", label: uiLanguage.value === "zh" ? "Switch to English" : "切换到中文", shortcut: "-" },
]);

const filteredPaletteCommands = computed(() => {
  const q = paletteQuery.value.trim().toLowerCase();
  if (!q) return paletteCommands.value;
  return paletteCommands.value.filter(
    (item) => item.label.toLowerCase().includes(q) || item.shortcut.toLowerCase().includes(q),
  );
});

const executePaletteAt = (index: number): void => {
  const list = filteredPaletteCommands.value;
  if (list.length === 0) return;
  const safeIndex = Math.max(0, Math.min(index, list.length - 1));
  runCommand(list[safeIndex].id);
  showCommandPalette.value = false;
};

const outlineItems = computed<OutlineItem[]>(() => {
  const lines = content.value.split("\n");
  const items: OutlineItem[] = [];
  let pos = 0;

  lines.forEach((line, index) => {
    const match = line.match(/^(#{1,6})\s+(.*)$/);
    if (match) {
      items.push({
        id: `heading-${index}`,
        level: match[1].length,
        title: match[2].trim() || "(Untitled)",
        pos,
        line: index,
      });
    }
    pos += line.length + 1;
  });

  return items;
});

const visibleOutlineItems = computed<VisibleOutlineItem[]>(() => {
  const items = outlineItems.value;
  const visible: VisibleOutlineItem[] = [];
  const collapsedLevels: number[] = [];

  for (let i = 0; i < items.length; i += 1) {
    const item = items[i];
    while (collapsedLevels.length > 0 && item.level <= collapsedLevels[collapsedLevels.length - 1]) {
      collapsedLevels.pop();
    }
    const hiddenByAncestor = collapsedLevels.length > 0;
    const next = items[i + 1];
    const hasChildren = Boolean(next && next.level > item.level);
    const collapsed = Boolean(collapsedOutlineMap.value[item.id]);

    if (!hiddenByAncestor) {
      visible.push({ ...item, hasChildren, collapsed });
    }

    if (collapsed && hasChildren) {
      collapsedLevels.push(item.level);
    }
  }

  return visible;
});

const filteredOutlineItems = computed(() => {
  const q = outlineQuery.value.trim().toLowerCase();
  if (!q) return visibleOutlineItems.value;
  return visibleOutlineItems.value.filter((item) => item.title.toLowerCase().includes(q));
});

const filteredRecentFiles = computed(() => {
  const q = recentQuery.value.trim().toLowerCase();
  if (!q) return recentFiles.value;
  return recentFiles.value.filter((item) => {
    return item.name.toLowerCase().includes(q) || item.path.toLowerCase().includes(q);
  });
});

const outlineHasCollapsibleItems = computed(() => {
  const items = outlineItems.value;
  for (let i = 0; i < items.length - 1; i += 1) {
    if (items[i + 1].level > items[i].level) return true;
  }
  return false;
});

const activeOutlineId = computed(() => {
  if (outlineItems.value.length === 0) return "";

  if (viewMode.value === "preview" && previewActiveLine.value >= 0) {
    let activeByPreview = outlineItems.value[0].id;
    for (const item of outlineItems.value) {
      if (item.line <= previewActiveLine.value) {
        activeByPreview = item.id;
      } else {
        break;
      }
    }
    return activeByPreview;
  }

  let active = outlineItems.value[0].id;
  for (const item of outlineItems.value) {
    if (item.pos <= cursorPos.value) {
      active = item.id;
    } else {
      break;
    }
  }
  return active;
});

const updatePreviewActiveLineFromScroll = (): void => {
  if (!previewRef.value) return;
  const anchors = previewRef.value.querySelectorAll<HTMLElement>("[data-source-line]");
  if (anchors.length === 0) {
    previewActiveLine.value = -1;
    return;
  }
  const markerY = previewRef.value.scrollTop + 12;
  let line = 0;
  anchors.forEach((node) => {
    const raw = node.getAttribute("data-source-line");
    if (!raw) return;
    const parsed = Number.parseInt(raw, 10);
    if (Number.isNaN(parsed)) return;
    if (node.offsetTop <= markerY) line = parsed;
  });
  previewActiveLine.value = line;
};

const toggleOutlineCollapse = (item: VisibleOutlineItem): void => {
  if (!item.hasChildren) return;
  collapsedOutlineMap.value = {
    ...collapsedOutlineMap.value,
    [item.id]: !item.collapsed,
  };
};

const collapseAllOutline = (): void => {
  const items = outlineItems.value;
  const next: Record<string, boolean> = {};
  for (let i = 0; i < items.length - 1; i += 1) {
    if (items[i + 1].level > items[i].level) {
      next[items[i].id] = true;
    }
  }
  collapsedOutlineMap.value = next;
};

const expandAllOutline = (): void => {
  collapsedOutlineMap.value = {};
};

const isWailsRuntime = (): boolean =>
  typeof window !== "undefined" && typeof (window as unknown as { go?: unknown }).go === "object";

const resolvePreviewImages = async (): Promise<void> => {
  if (!isWailsRuntime()) return;

  const html = md.render(content.value);
  const srcSet = new Set<string>();
  html.replace(/<img[^>]*src="([^"]+)"[^>]*>/gi, (_m, src) => {
    srcSet.add(String(src));
    return _m;
  });

  for (const src of srcSet) {
    if (imagePreviewMap.value[src] || loadingImageSources.has(src)) continue;
    loadingImageSources.add(src);
    try {
      const dataURL = await ResolveImageDataURL(filePath.value, src);
      if (dataURL) {
        imagePreviewMap.value = {
          ...imagePreviewMap.value,
          [src]: dataURL,
          [encodeURI(src)]: dataURL,
        };
        imageStatus.value = `Image: resolved ${src}`;
      } else {
        imageStatus.value = `Image: resolve empty ${src}`;
        updateStatus(`Image resolve failed: ${src} (empty data)`);
      }
    } catch (error) {
      imageStatus.value = `Image: resolve failed ${src}`;
      updateStatus(`Image resolve failed: ${src} :: ${String(error)}`);
    } finally {
      loadingImageSources.delete(src);
    }
  }
};

const updateStatus = (text: string): void => {
  statusText.value = text;
};

const normalizeErrorText = (error: unknown): string => {
  const raw = String(error || "unknown error").replace(/^Error:\s*/i, "").trim();
  return raw || "unknown error";
};

const recordAutosaveError = (source: AutosaveErrorEntry["source"], message: string): void => {
  const normalized = message.trim() || "unknown error";
  autosaveErrorText.value = normalized;
  const entry: AutosaveErrorEntry = {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    at: new Date().toLocaleString(),
    source,
    message: normalized,
  };
  autosaveErrorHistory.value = [entry, ...autosaveErrorHistory.value].slice(0, 20);
  autosaveErrorActiveId.value = entry.id;
};

const markSavedNow = (): void => {
  autosaveState.value = "saved";
  autosaveErrorText.value = "";
  autosaveAt.value = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
};

const updateCursorStatus = (): void => {
  if (!editorView) return;
  const head = editorView.state.selection.main.head;
  const line = editorView.state.doc.lineAt(head);
  cursorPos.value = head;
  cursorLine.value = line.number;
  cursorCol.value = head - line.from + 1;
};

const insertTextAtCursor = (text: string): void => {
  if (!editorView) return;
  const sel = editorView.state.selection.main;
  editorView.dispatch({
    changes: { from: sel.from, to: sel.to, insert: text },
    selection: EditorSelection.cursor(sel.from + text.length),
  });
  editorView.focus();
};

const fileToDataURL = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(reader.error || new Error("read file failed"));
    reader.readAsDataURL(file);
  });

const collectImageFiles = (files: FileList | null): File[] => {
  if (!files || files.length === 0) return [];
  const images: File[] = [];
  for (let i = 0; i < files.length; i += 1) {
    const f = files.item(i);
    if (f && f.type.startsWith("image/")) images.push(f);
  }
  return images;
};

const saveAndInsertImage = async (file: File): Promise<void> => {
  if (!file.type.startsWith("image/")) {
    imageStatus.value = `Image: unsupported type ${file.type}`;
    updateStatus("Only image files are supported");
    return;
  }

  if (!isWailsRuntime()) {
    const dataURL = await fileToDataURL(file);
    insertTextAtCursor(`\n![${file.name || "image"}](${dataURL})\n`);
    imageStatus.value = "Image: inserted as data URL (browser mode)";
    updateStatus("Inserted image as data URL (browser mode)");
    return;
  }

  try {
    const dataURL = await fileToDataURL(file);
    const result = await SaveImageAsset({
      documentPath: filePath.value,
      fileName: file.name || "",
      dataURL,
    });
    if (!result) {
      imageStatus.value = "Image: save cancelled";
      updateStatus("Image save cancelled");
      return;
    }
    const mdPath = result.relativePath || result.absolutePath;
    const absPath = result.absolutePath || mdPath;
    const encodedPath = encodeURI(mdPath);
    let decodedPath = mdPath;
    try {
      decodedPath = decodeURI(mdPath);
    } catch {
      // ignore decode issues
    }
    const fileURL = absPath.startsWith("/") ? `file://${encodeURI(absPath)}` : absPath;
    imagePreviewMap.value = {
      ...imagePreviewMap.value,
      [mdPath]: dataURL,
      [encodedPath]: dataURL,
      [decodedPath]: dataURL,
      [absPath]: dataURL,
      [fileURL]: dataURL,
    };
    insertTextAtCursor(`\n![${file.name || "image"}](<${mdPath}>)\n`);
    imageStatus.value = `Image: cached ${mdPath}`;
    updateStatus(`Image saved + preview cached: ${mdPath}`);
  } catch (error) {
    imageStatus.value = `Image: save failed ${String(error)}`;
    updateStatus(`Image save failed: ${String(error)}`);
  }
};

const saveAndInsertImages = async (files: File[]): Promise<void> => {
  if (files.length === 0) return;
  for (const file of files) {
    await saveAndInsertImage(file);
  }
};

const handleEditorPaste = (event: ClipboardEvent): void => {
  const items = event.clipboardData?.items;
  if (!items) return;
  const images: File[] = [];
  for (const item of items) {
    if (item.type.startsWith("image/")) {
      const file = item.getAsFile();
      if (file) images.push(file);
    }
  }
  if (images.length === 0) return;
  event.preventDefault();
  void saveAndInsertImages(images);
};

const handleEditorDrop = (event: DragEvent): void => {
  const images = collectImageFiles(event.dataTransfer?.files ?? null);
  if (images.length === 0) return;
  event.preventDefault();
  void saveAndInsertImages(images);
};

const handleEditorDragOver = (event: DragEvent): void => {
  event.preventDefault();
};

const handlePreviewDrop = (event: DragEvent): void => {
  const images = collectImageFiles(event.dataTransfer?.files ?? null);
  if (images.length === 0) return;
  event.preventDefault();
  editorView?.focus();
  void saveAndInsertImages(images);
};

const handlePreviewDragOver = (event: DragEvent): void => {
  event.preventDefault();
};

const applyWrap = (prefix: string, suffix: string): void => {
  if (!editorView) return;
  const sel = editorView.state.selection.main;
  const selected = editorView.state.doc.sliceString(sel.from, sel.to);

  if (selected && selected.startsWith(prefix) && selected.endsWith(suffix)) {
    const unwrapped = selected.slice(prefix.length, selected.length - suffix.length);
    editorView.dispatch({
      changes: { from: sel.from, to: sel.to, insert: unwrapped },
      selection: EditorSelection.range(sel.from, sel.from + unwrapped.length),
    });
    editorView.focus();
    return;
  }

  const insert = `${prefix}${selected || "text"}${suffix}`;
  const cursor = selected ? sel.from + insert.length : sel.from + prefix.length;
  editorView.dispatch({
    changes: { from: sel.from, to: sel.to, insert },
    selection: EditorSelection.cursor(cursor),
  });
  editorView.focus();
};

const applyLinePrefix = (prefix: string): void => {
  if (!editorView) return;
  const sel = editorView.state.selection.main;
  const fromLine = editorView.state.doc.lineAt(sel.from);
  const toLine = editorView.state.doc.lineAt(sel.to);
  const from = fromLine.from;
  const to = toLine.to;
  const text = editorView.state.doc.sliceString(from, to);
  const lines = text.split("\n");
  const allPrefixed = lines.every((line) => line.startsWith(prefix));
  const replaced = lines
    .map((line) => (allPrefixed ? line.slice(prefix.length) : `${prefix}${line}`))
    .join("\n");
  editorView.dispatch({
    changes: { from, to, insert: replaced },
    selection: EditorSelection.range(from, from + replaced.length),
  });
  editorView.focus();
};

const applyHeading = (level: number): void => {
  if (!editorView) return;
  const sel = editorView.state.selection.main;
  const fromLine = editorView.state.doc.lineAt(sel.from);
  const toLine = editorView.state.doc.lineAt(sel.to);
  const from = fromLine.from;
  const to = toLine.to;
  const text = editorView.state.doc.sliceString(from, to);
  const targetPrefix = `${"#".repeat(level)} `;
  const lines = text.split("\n");
  const allSameLevel = lines.every((line) => line.startsWith(targetPrefix));
  const replaced = lines
    .map((line) => {
      const withoutHeading = line.replace(/^#{1,6}\s+/, "");
      return allSameLevel ? withoutHeading : `${targetPrefix}${withoutHeading}`;
    })
    .join("\n");

  editorView.dispatch({
    changes: { from, to, insert: replaced },
    selection: EditorSelection.range(from, from + replaced.length),
  });
  editorView.focus();
};

const applyCodeBlock = (): void => {
  if (!editorView) return;
  const sel = editorView.state.selection.main;
  const selected = editorView.state.doc.sliceString(sel.from, sel.to);

  if (selected) {
    const fenced = selected.match(/^```[^\n]*\n([\s\S]*?)\n```$/);
    if (fenced) {
      const unwrapped = fenced[1];
      editorView.dispatch({
        changes: { from: sel.from, to: sel.to, insert: unwrapped },
        selection: EditorSelection.range(sel.from, sel.from + unwrapped.length),
      });
      editorView.focus();
      return;
    }
  }

  const insert = `\n\`\`\`\n${selected || "code"}\n\`\`\`\n`;
  editorView.dispatch({
    changes: { from: sel.from, to: sel.to, insert },
    selection: EditorSelection.cursor(sel.from + insert.length),
  });
  editorView.focus();
};

const continueMarkdownListOnEnter = (view: EditorView): boolean => {
  const sel = view.state.selection.main;
  if (!sel.empty) return false;

  const line = view.state.doc.lineAt(sel.from);
  if (sel.from !== line.to) return false;

  const lineText = line.text;

  const emptyUnordered = lineText.match(/^(\s*)([-*+])\s*(\[[ xX]\]\s*)?$/);
  if (emptyUnordered) {
    const indent = emptyUnordered[1];
    view.dispatch({
      changes: { from: line.from, to: line.to, insert: indent },
      selection: EditorSelection.cursor(line.from + indent.length),
    });
    return true;
  }

  const emptyOrdered = lineText.match(/^(\s*)(\d+)\.\s*$/);
  if (emptyOrdered) {
    const indent = emptyOrdered[1];
    view.dispatch({
      changes: { from: line.from, to: line.to, insert: indent },
      selection: EditorSelection.cursor(line.from + indent.length),
    });
    return true;
  }

  const unordered = lineText.match(/^(\s*)([-*+])\s+(\[[ xX]\]\s+)?(.+)$/);
  if (unordered) {
    const indent = unordered[1];
    const bullet = unordered[2];
    const taskPrefix = unordered[3] ? "[ ] " : "";
    const insert = `\n${indent}${bullet} ${taskPrefix}`;
    view.dispatch({
      changes: { from: sel.from, to: sel.to, insert },
      selection: EditorSelection.cursor(sel.from + insert.length),
    });
    return true;
  }

  const ordered = lineText.match(/^(\s*)(\d+)\.\s+(.+)$/);
  if (ordered) {
    const indent = ordered[1];
    const currentNo = Number.parseInt(ordered[2], 10);
    if (Number.isNaN(currentNo)) return false;
    const nextNo = currentNo + 1;
    const insert = `\n${indent}${nextNo}. `;
    view.dispatch({
      changes: { from: sel.from, to: sel.to, insert },
      selection: EditorSelection.cursor(sel.from + insert.length),
    });
    return true;
  }

  return false;
};

const indentSelectionWithSpaces = (view: EditorView): boolean => {
  const sel = view.state.selection.main;
  const fromLine = view.state.doc.lineAt(sel.from);
  const toLine = view.state.doc.lineAt(sel.to);
  const from = fromLine.from;
  const to = toLine.to;
  const text = view.state.doc.sliceString(from, to);
  const replaced = text
    .split("\n")
    .map((line) => `  ${line}`)
    .join("\n");
  view.dispatch({
    changes: { from, to, insert: replaced },
    selection: EditorSelection.range(from, from + replaced.length),
  });
  return true;
};

const outdentSelectionWithSpaces = (view: EditorView): boolean => {
  const sel = view.state.selection.main;
  const fromLine = view.state.doc.lineAt(sel.from);
  const toLine = view.state.doc.lineAt(sel.to);
  const from = fromLine.from;
  const to = toLine.to;
  const text = view.state.doc.sliceString(from, to);
  const replaced = text
    .split("\n")
    .map((line) => {
      if (line.startsWith("  ")) return line.slice(2);
      if (line.startsWith("\t")) return line.slice(1);
      if (line.startsWith(" ")) return line.slice(1);
      return line;
    })
    .join("\n");
  view.dispatch({
    changes: { from, to, insert: replaced },
    selection: EditorSelection.range(from, from + replaced.length),
  });
  return true;
};

const persistRecentFiles = (): void => {
  localStorage.setItem(RECENT_FILES_KEY, JSON.stringify(recentFiles.value));
};

const getSavedRecentOrder = (): string[] => {
  try {
    const raw = localStorage.getItem(RECENT_ORDER_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as string[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is string => typeof item === "string" && item.trim() !== "");
  } catch {
    return [];
  }
};

const getPinnedRecentPaths = (): string[] => {
  try {
    const raw = localStorage.getItem(RECENT_PINNED_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as string[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is string => typeof item === "string" && item.trim() !== "");
  } catch {
    return [];
  }
};

const persistRecentOrder = (): void => {
  const order = recentFiles.value.map((item) => item.path);
  localStorage.setItem(RECENT_ORDER_KEY, JSON.stringify(order));
};

const persistPinnedRecentPaths = (): void => {
  localStorage.setItem(RECENT_PINNED_KEY, JSON.stringify(pinnedRecentPaths.value));
};

const sortRecentBySavedOrder = (items: RecentFile[]): RecentFile[] => {
  const order = getSavedRecentOrder();
  const pinnedSet = new Set(pinnedRecentPaths.value);
  if (order.length === 0) {
    return [...items].sort((a, b) => {
      const ap = pinnedSet.has(a.path) ? 0 : 1;
      const bp = pinnedSet.has(b.path) ? 0 : 1;
      return ap - bp;
    });
  }
  const indexMap = new Map<string, number>();
  order.forEach((path, idx) => indexMap.set(path, idx));
  return [...items].sort((a, b) => {
    const ap = pinnedSet.has(a.path) ? 0 : 1;
    const bp = pinnedSet.has(b.path) ? 0 : 1;
    if (ap !== bp) return ap - bp;
    const ai = indexMap.has(a.path) ? (indexMap.get(a.path) as number) : Number.MAX_SAFE_INTEGER;
    const bi = indexMap.has(b.path) ? (indexMap.get(b.path) as number) : Number.MAX_SAFE_INTEGER;
    if (ai !== bi) return ai - bi;
    return 0;
  });
};

const loadRecentFiles = async (): Promise<void> => {
  pinnedRecentPaths.value = getPinnedRecentPaths();

  if (isWailsRuntime()) {
    try {
      const items = await ListRecentFiles(10);
      recentFiles.value = sortRecentBySavedOrder(items ?? []);
      const validPathSet = new Set(recentFiles.value.map((item) => item.path));
      pinnedRecentPaths.value = pinnedRecentPaths.value.filter((path) => validPathSet.has(path));
      persistPinnedRecentPaths();
      persistRecentOrder();
      return;
    } catch {
      recentFiles.value = [];
      return;
    }
  }

  try {
    const raw = localStorage.getItem(RECENT_FILES_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw) as RecentFile[];
    recentFiles.value = sortRecentBySavedOrder(parsed.filter((item) => item.path && item.name).slice(0, 10));
    const validPathSet = new Set(recentFiles.value.map((item) => item.path));
    pinnedRecentPaths.value = pinnedRecentPaths.value.filter((path) => validPathSet.has(path));
    persistPinnedRecentPaths();
    persistRecentOrder();
  } catch {
    recentFiles.value = [];
  }
};

const addRecentFile = async (path: string, name: string): Promise<void> => {
  if (!path || !name) return;

  if (isWailsRuntime()) {
    try {
      await AddRecentFile(path, name);
      const items = await ListRecentFiles(10);
      recentFiles.value = sortRecentBySavedOrder(items ?? []);
      persistRecentOrder();
      return;
    } catch {
      return;
    }
  }

  recentFiles.value = [{ path, name }, ...recentFiles.value.filter((item) => item.path !== path)].slice(0, 10);
  persistRecentFiles();
  persistRecentOrder();
};

const removeRecentFileByPath = async (path: string, nameForStatus = "", silent = false): Promise<void> => {
  if (!path) return;

  if (isWailsRuntime()) {
    try {
      await RemoveRecentFile(path);
      const items = await ListRecentFiles(10);
      recentFiles.value = sortRecentBySavedOrder(items ?? []);
      persistRecentOrder();
      if (!silent) updateStatus(`Removed recent: ${nameForStatus || path}`);
      return;
    } catch {
      if (!silent) updateStatus(`Failed to remove recent: ${nameForStatus || path}`);
      return;
    }
  }

  recentFiles.value = recentFiles.value.filter((f) => f.path !== path);
  pinnedRecentPaths.value = pinnedRecentPaths.value.filter((p) => p !== path);
  persistPinnedRecentPaths();
  persistRecentFiles();
  persistRecentOrder();
  if (!silent) updateStatus(`Removed recent: ${nameForStatus || path}`);
};

const removeRecentFile = async (item: RecentFile): Promise<void> => {
  await removeRecentFileByPath(item.path, item.name, false);
};

const isRecentPinned = (path: string): boolean => pinnedRecentPaths.value.includes(path);

const togglePinRecentFile = (item: RecentFile): void => {
  const path = item.path;
  if (!path) return;
  if (isRecentPinned(path)) {
    pinnedRecentPaths.value = pinnedRecentPaths.value.filter((p) => p !== path);
    updateStatus(`Unpinned recent: ${item.name}`);
  } else {
    pinnedRecentPaths.value = [path, ...pinnedRecentPaths.value.filter((p) => p !== path)];
    updateStatus(`Pinned recent: ${item.name}`);
  }
  persistPinnedRecentPaths();
  recentFiles.value = sortRecentBySavedOrder(recentFiles.value);
};

const clearAllRecentFiles = async (): Promise<void> => {
  if (recentFiles.value.length === 0) return;

  if (isWailsRuntime()) {
    try {
      await ClearRecentFiles();
      recentFiles.value = [];
      pinnedRecentPaths.value = [];
      persistPinnedRecentPaths();
      persistRecentOrder();
      updateStatus("Recent files cleared");
      return;
    } catch {
      updateStatus("Failed to clear recent files");
      return;
    }
  }

  recentFiles.value = [];
  pinnedRecentPaths.value = [];
  persistPinnedRecentPaths();
  persistRecentFiles();
  persistRecentOrder();
  updateStatus("Recent files cleared");
};

const moveRecentFile = (fromPath: string, toPath: string): void => {
  if (!fromPath || !toPath || fromPath === toPath) return;
  const fromIdx = recentFiles.value.findIndex((item) => item.path === fromPath);
  const toIdx = recentFiles.value.findIndex((item) => item.path === toPath);
  if (fromIdx < 0 || toIdx < 0) return;
  const next = [...recentFiles.value];
  const [moved] = next.splice(fromIdx, 1);
  next.splice(toIdx, 0, moved);
  recentFiles.value = next;
  persistRecentOrder();
};

const onRecentDragStart = (event: DragEvent, path: string): void => {
  draggingRecentPath.value = path;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", path);
  }
};

const onRecentDragOver = (event: DragEvent): void => {
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "move";
  }
};

const onRecentDrop = (event: DragEvent, targetPath: string): void => {
  event.preventDefault();
  const sourcePath = draggingRecentPath.value || event.dataTransfer?.getData("text/plain") || "";
  moveRecentFile(sourcePath, targetPath);
  draggingRecentPath.value = "";
};

const onRecentDragEnd = (): void => {
  draggingRecentPath.value = "";
};

const setDocument = (nextContent: string, nextName: string, nextPath: string, nextDirty = false): void => {
  if (editorView && editorView.state.doc.toString() !== nextContent) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: nextContent },
      selection: EditorSelection.cursor(0),
    });
  }
  content.value = nextContent;
  fileName.value = nextName;
  filePath.value = nextPath;
  dirty.value = nextDirty;
  autosaveState.value = "idle";
  autosaveAt.value = "";
};

const restoreDraft = (): boolean => {
  const draftContent = localStorage.getItem(DRAFT_CONTENT_KEY);
  if (!draftContent) return false;
  const draftName = localStorage.getItem(DRAFT_NAME_KEY) || "untitled.md";
  const draftPath = localStorage.getItem(DRAFT_PATH_KEY) || "";
  setDocument(draftContent, draftName, draftPath, true);
  syncActiveTabFromState();
  updateStatus("Draft restored from local cache");
  return true;
};

const restoreLastFile = async (): Promise<boolean> => {
  if (!isWailsRuntime()) return false;
  const lastPath = (localStorage.getItem(LAST_FILE_PATH_KEY) || "").trim();
  if (!lastPath) return false;
  try {
    const result = await OpenMarkdownFileAtPath(lastPath);
    if (!result) {
      localStorage.removeItem(LAST_FILE_PATH_KEY);
      return false;
    }
    setDocument(result.content, result.name, result.path);
    syncActiveTabFromState();
    updateStatus(`Restored last file: ${result.name}`);
    return true;
  } catch {
    localStorage.removeItem(LAST_FILE_PATH_KEY);
    return false;
  }
};

const restoreUIState = (): void => {
  const savedTheme = localStorage.getItem(UI_THEME_KEY);
  if (savedTheme === "dark") isDarkTheme.value = true;
  if (savedTheme === "light") isDarkTheme.value = false;

  const savedLang = localStorage.getItem(UI_LANG_KEY);
  if (savedLang === "zh" || savedLang === "en") {
    uiLanguage.value = savedLang;
  }

  const savedRedoY = localStorage.getItem(UI_SHORTCUT_REDO_Y_KEY);
  if (savedRedoY === "1") enableRedoWithY.value = true;
  if (savedRedoY === "0") enableRedoWithY.value = false;

  const savedZenShortcut = localStorage.getItem(UI_SHORTCUT_ZEN_KEY);
  if (savedZenShortcut === "1") enableZenShortcut.value = true;
  if (savedZenShortcut === "0") enableZenShortcut.value = false;

  try {
    const savedBindings = JSON.parse(localStorage.getItem(UI_SHORTCUT_BINDINGS_KEY) || "{}") as Record<string, unknown>;
    shortcutBindings.value = normalizeShortcutBindings(savedBindings);
    shortcutBindingsCommitted.value = { ...shortcutBindings.value };
  } catch {
    shortcutBindings.value = { ...DEFAULT_SHORTCUT_BINDINGS };
    shortcutBindingsCommitted.value = { ...DEFAULT_SHORTCUT_BINDINGS };
  }

  const savedSidebar = localStorage.getItem(UI_SIDEBAR_KEY);
  if (savedSidebar === "1") showSidebar.value = true;
  if (savedSidebar === "0") showSidebar.value = false;

  const savedZen = localStorage.getItem(UI_ZEN_KEY);
  if (savedZen === "1") isZenMode.value = true;
  if (savedZen === "0") isZenMode.value = false;

  const savedScrollSync = localStorage.getItem(UI_SCROLL_SYNC_KEY);
  if (savedScrollSync === "1") isScrollSyncEnabled.value = true;
  if (savedScrollSync === "0") isScrollSyncEnabled.value = false;

  const savedLineNumbers = localStorage.getItem(UI_LINE_NUMBERS_KEY);
  if (savedLineNumbers === "1") showLineNumbers.value = true;
  if (savedLineNumbers === "0") showLineNumbers.value = false;

  const savedWrapLines = localStorage.getItem(UI_WRAP_LINES_KEY);
  if (savedWrapLines === "1") wrapLines.value = true;
  if (savedWrapLines === "0") wrapLines.value = false;

  const savedEditorFontSize = Number.parseInt(localStorage.getItem(UI_EDITOR_FONT_SIZE_KEY) || "", 10);
  if (Number.isFinite(savedEditorFontSize)) {
    editorFontSize.value = Math.max(12, Math.min(22, savedEditorFontSize));
  }
  const savedEditorFontFamily = localStorage.getItem(UI_EDITOR_FONT_FAMILY_KEY) || "";
  if (EDITOR_FONT_FAMILY_OPTIONS.some((item) => item.value === savedEditorFontFamily)) {
    editorFontFamily.value = savedEditorFontFamily;
  }

  const savedStatusbar = localStorage.getItem(UI_STATUSBAR_KEY);
  if (savedStatusbar === "1") showStatusbar.value = true;
  if (savedStatusbar === "0") showStatusbar.value = false;

  const savedAutosave = localStorage.getItem(UI_AUTOSAVE_KEY);
  if (savedAutosave === "1") isAutosaveEnabled.value = true;
  if (savedAutosave === "0") isAutosaveEnabled.value = false;

  const savedAutosaveInterval = Number.parseInt(localStorage.getItem(UI_AUTOSAVE_INTERVAL_KEY) || "", 10);
  if (
    Number.isFinite(savedAutosaveInterval) &&
    AUTOSAVE_INTERVAL_OPTIONS.includes(savedAutosaveInterval as (typeof AUTOSAVE_INTERVAL_OPTIONS)[number])
  ) {
    autosaveIntervalMs.value = savedAutosaveInterval;
  }

  try {
    const savedErrors = JSON.parse(localStorage.getItem(UI_AUTOSAVE_ERRORS_KEY) || "[]") as AutosaveErrorEntry[];
    if (Array.isArray(savedErrors)) {
      autosaveErrorHistory.value = savedErrors
        .filter(
          (item) =>
            item &&
            typeof item.id === "string" &&
            typeof item.at === "string" &&
            (item.source === "autosave" || item.source === "save" || item.source === "saveAs") &&
            typeof item.message === "string",
        )
        .slice(0, 20);
      if (autosaveErrorHistory.value.length > 0) {
        autosaveErrorActiveId.value = autosaveErrorHistory.value[0].id;
      }
    }
  } catch {
    autosaveErrorHistory.value = [];
    autosaveErrorActiveId.value = "";
  }

  const savedAutosaveErrorSource = localStorage.getItem(UI_AUTOSAVE_ERROR_SOURCE_KEY);
  if (savedAutosaveErrorSource === "all" || savedAutosaveErrorSource === "autosave" || savedAutosaveErrorSource === "save" || savedAutosaveErrorSource === "saveAs") {
    autosaveErrorSourceFilter.value = savedAutosaveErrorSource;
  }

  const savedAutosaveErrorQuery = localStorage.getItem(UI_AUTOSAVE_ERROR_QUERY_KEY);
  if (typeof savedAutosaveErrorQuery === "string") {
    autosaveErrorQuery.value = savedAutosaveErrorQuery;
  }

  const savedAutosaveErrorSort = localStorage.getItem(UI_AUTOSAVE_ERROR_SORT_KEY);
  if (savedAutosaveErrorSort === "asc" || savedAutosaveErrorSort === "desc") {
    autosaveErrorSortOrder.value = savedAutosaveErrorSort;
  }

  const savedViewMode = localStorage.getItem(UI_VIEWMODE_KEY);
  if (savedViewMode === "split" || savedViewMode === "edit" || savedViewMode === "preview") {
    viewMode.value = savedViewMode;
  }

  const savedSplitRatio = Number.parseFloat(localStorage.getItem(UI_SPLIT_RATIO_KEY) || "");
  if (Number.isFinite(savedSplitRatio)) {
    splitRatio.value = Math.max(20, Math.min(80, savedSplitRatio));
  }

  const savedSidebarWidth = Number.parseFloat(localStorage.getItem(UI_SIDEBAR_WIDTH_KEY) || "");
  if (Number.isFinite(savedSidebarWidth)) {
    sidebarWidth.value = Math.max(180, Math.min(420, savedSidebarWidth));
  }

  try {
    const savedCollapsed = JSON.parse(localStorage.getItem(UI_OUTLINE_COLLAPSE_KEY) || "{}") as Record<string, boolean>;
    if (savedCollapsed && typeof savedCollapsed === "object") {
      collapsedOutlineMap.value = savedCollapsed;
    }
  } catch {
    collapsedOutlineMap.value = {};
  }
};

const scheduleAutosave = (): void => {
  if (autosaveTimer) clearTimeout(autosaveTimer);
  if (dirty.value && isAutosaveEnabled.value) autosaveState.value = "pending";

  autosaveTimer = setTimeout(async () => {
    localStorage.setItem(DRAFT_CONTENT_KEY, content.value);
    localStorage.setItem(DRAFT_NAME_KEY, fileName.value);
    localStorage.setItem(DRAFT_PATH_KEY, filePath.value);

    if (!isAutosaveEnabled.value || !isWailsRuntime() || !filePath.value || !dirty.value) return;

    try {
      autosaveState.value = "saving";
      const result = await SaveMarkdownFile({
        path: filePath.value,
        fileName: fileName.value,
        content: content.value,
      });
      if (!result) return;
      filePath.value = result.path;
      fileName.value = result.name;
      dirty.value = false;
      markSavedNow();
      void addRecentFile(result.path, result.name);
      updateStatus(`Autosaved ${result.name}`);
    } catch (error) {
      autosaveState.value = "error";
      recordAutosaveError("autosave", normalizeErrorText(error));
      updateStatus(`Autosave failed: ${autosaveErrorText.value}`);
    }
  }, autosaveIntervalMs.value);
};

const syncPreviewFromEditor = (): void => {
  if (!isScrollSyncEnabled.value) return;
  if (!editorScrollElement || !previewRef.value || syncingFromPreview) return;
  if (performance.now() < ignoreEditorScrollUntil) return;
  const sourceMax = editorScrollElement.scrollHeight - editorScrollElement.clientHeight;
  const targetMax = previewRef.value.scrollHeight - previewRef.value.clientHeight;
  if (sourceMax <= 0 || targetMax <= 0) return;

  syncingFromEditor = true;
  const ratio = editorScrollElement.scrollTop / sourceMax;
  const next = ratio * targetMax;
  if (Math.abs(previewRef.value.scrollTop - next) > 1) {
    ignorePreviewScrollUntil = performance.now() + 140;
    previewRef.value.scrollTop = next;
  }
  requestAnimationFrame(() => {
    syncingFromEditor = false;
  });
};

const syncEditorFromPreview = (): void => {
  updatePreviewActiveLineFromScroll();
  if (!isScrollSyncEnabled.value) return;
  if (!editorScrollElement || !previewRef.value || syncingFromEditor) return;
  if (performance.now() < ignorePreviewScrollUntil) return;
  const sourceMax = previewRef.value.scrollHeight - previewRef.value.clientHeight;
  const targetMax = editorScrollElement.scrollHeight - editorScrollElement.clientHeight;
  if (sourceMax <= 0 || targetMax <= 0) return;

  syncingFromPreview = true;
  const ratio = previewRef.value.scrollTop / sourceMax;
  const next = ratio * targetMax;
  if (Math.abs(editorScrollElement.scrollTop - next) > 1) {
    ignoreEditorScrollUntil = performance.now() + 140;
    editorScrollElement.scrollTop = next;
  }
  requestAnimationFrame(() => {
    syncingFromPreview = false;
  });
};

const updateSplitRatioFromClientX = (clientX: number): void => {
  const host = docZoneRef.value;
  if (!host) return;
  const rect = host.getBoundingClientRect();
  if (rect.width <= 0) return;
  const next = ((clientX - rect.left) / rect.width) * 100;
  splitRatio.value = Math.max(20, Math.min(80, next));
};

const onSplitDragMove = (event: MouseEvent): void => {
  event.preventDefault();
  updateSplitRatioFromClientX(event.clientX);
};

const onSplitDragEnd = (): void => {
  window.removeEventListener("mousemove", onSplitDragMove);
  window.removeEventListener("mouseup", onSplitDragEnd);
};

const startSplitDrag = (event: MouseEvent): void => {
  if (viewMode.value !== "split") return;
  event.preventDefault();
  updateSplitRatioFromClientX(event.clientX);
  window.addEventListener("mousemove", onSplitDragMove);
  window.addEventListener("mouseup", onSplitDragEnd);
};

const resetSplitRatio = (): void => {
  splitRatio.value = DEFAULT_SPLIT_RATIO;
  updateStatus("Split ratio reset");
};

const updateSidebarWidthFromClientX = (clientX: number): void => {
  const host = workspaceRef.value;
  if (!host) return;
  const rect = host.getBoundingClientRect();
  if (rect.width <= 0) return;
  const next = clientX - rect.left;
  sidebarWidth.value = Math.max(180, Math.min(420, next));
};

const onSidebarDragMove = (event: MouseEvent): void => {
  event.preventDefault();
  updateSidebarWidthFromClientX(event.clientX);
};

const onSidebarDragEnd = (): void => {
  window.removeEventListener("mousemove", onSidebarDragMove);
  window.removeEventListener("mouseup", onSidebarDragEnd);
};

const startSidebarDrag = (event: MouseEvent): void => {
  if (!showSidebarSplitter.value) return;
  event.preventDefault();
  updateSidebarWidthFromClientX(event.clientX);
  window.addEventListener("mousemove", onSidebarDragMove);
  window.addEventListener("mouseup", onSidebarDragEnd);
};

const resetSidebarWidth = (): void => {
  sidebarWidth.value = DEFAULT_SIDEBAR_WIDTH;
  updateStatus("Sidebar width reset");
};

const resetLayoutSizes = (): void => {
  resetSplitRatio();
  resetSidebarWidth();
  updateStatus("Layout sizes reset");
};

const jumpToPosition = (pos: number): void => {
  if (!editorView) return;
  const maxPos = editorView.state.doc.length;
  const safePos = Math.max(0, Math.min(pos, maxPos));
  editorView.dispatch({
    selection: EditorSelection.cursor(safePos),
    effects: EditorView.scrollIntoView(safePos, { y: "center" }),
  });
  editorView.focus();
};

const jumpToOutlineItem = (item: OutlineItem): void => {
  if (viewMode.value === "preview" && previewRef.value) {
    const anchor = previewRef.value.querySelector<HTMLElement>(`[data-source-line="${item.line}"]`);
    if (anchor) {
      const next = Math.max(0, anchor.offsetTop - 12);
      previewRef.value.scrollTop = next;
      previewActiveLine.value = item.line;
      return;
    }
  }
  jumpToPosition(item.pos);
};

const jumpToLine = (lineZeroBased: number): void => {
  if (!editorView) return;
  const docLines = editorView.state.doc.lines;
  const lineNo = Math.max(1, Math.min(docLines, lineZeroBased + 1));
  const line = editorView.state.doc.line(lineNo);
  jumpToPosition(line.from);
};

const goToLine = (): void => {
  if (!editorView) return;
  const docLines = editorView.state.doc.lines;
  const raw = window.prompt(`Go to line (1-${docLines})`, String(cursorLine.value));
  if (!raw) return;
  const lineNo = Number.parseInt(raw.trim(), 10);
  if (Number.isNaN(lineNo)) {
    updateStatus("Invalid line number");
    return;
  }
  const bounded = Math.max(1, Math.min(docLines, lineNo));
  jumpToLine(bounded - 1);
  updateStatus(`Jumped to line ${bounded}`);
};

const handlePreviewClick = (event: MouseEvent): void => {
  const target = event.target as HTMLElement | null;
  if (!target) return;

  const link = target.closest<HTMLAnchorElement>("a[href]");
  if (link) {
    const href = (link.getAttribute("href") || "").trim();
    if (href && !href.startsWith("#")) {
      event.preventDefault();
      const isExternal = /^(https?:|mailto:)/i.test(href);
      if (isExternal) {
        if (isWailsRuntime()) {
          BrowserOpenURL(href);
        } else {
          window.open(href, "_blank", "noopener,noreferrer");
        }
        updateStatus(`Opened link: ${href}`);
      }
    }
    return;
  }

  if (target.closest("input[type='checkbox']")) return;
  const host = target.closest<HTMLElement>("[data-source-line]");
  if (!host) return;
  const raw = host.getAttribute("data-source-line");
  if (!raw) return;
  const line = Number.parseInt(raw, 10);
  if (Number.isNaN(line)) return;
  previewActiveLine.value = line;
  jumpToLine(line);
};

const toggleTaskAtLine = (lineZeroBased: number, checked: boolean): void => {
  if (!editorView) return;
  const lineNo = Math.max(1, Math.min(editorView.state.doc.lines, lineZeroBased + 1));
  const line = editorView.state.doc.line(lineNo);
  const original = line.text;
  const next = original.replace(
    /^(\s*(?:[-*+]|\d+\.)\s+\[)( |x|X)(\])/,
    (_m, p1: string, _p2: string, p3: string) => `${p1}${checked ? "x" : " "}${p3}`,
  );
  if (next === original) return;
  editorView.dispatch({
    changes: { from: line.from, to: line.to, insert: next },
  });
};

const handlePreviewChange = (event: Event): void => {
  const input = event.target as HTMLInputElement | null;
  if (!input || input.type !== "checkbox") return;
  const host = input.closest<HTMLElement>("[data-source-line]");
  if (!host) return;
  const raw = host.getAttribute("data-source-line");
  if (!raw) return;
  const line = Number.parseInt(raw, 10);
  if (Number.isNaN(line)) return;
  toggleTaskAtLine(line, input.checked);
  updateStatus(`Task ${input.checked ? "checked" : "unchecked"}`);
};

const runWithUnsavedGuard = (action: () => void | Promise<void>): boolean => {
  if (!dirty.value) {
    void action();
    return true;
  }
  pendingUnsavedAction = action;
  showUnsavedConfirm.value = true;
  updateStatus("Unsaved changes detected");
  return false;
};

const confirmDiscardAndContinue = (): void => {
  const action = pendingUnsavedAction;
  pendingUnsavedAction = null;
  showUnsavedConfirm.value = false;
  if (action) void action();
};

const cancelDiscard = (): void => {
  pendingUnsavedAction = null;
  showUnsavedConfirm.value = false;
  updateStatus("Operation cancelled: unsaved changes kept");
};

const requestDeleteConfirm = (text: string, action: () => void): void => {
  deleteConfirmText.value = text;
  pendingDeleteAction = action;
  showDeleteConfirm.value = true;
};

const confirmDeleteAction = (): void => {
  const action = pendingDeleteAction;
  pendingDeleteAction = null;
  showDeleteConfirm.value = false;
  if (action) action();
};

const cancelDeleteAction = (): void => {
  pendingDeleteAction = null;
  showDeleteConfirm.value = false;
  updateStatus("Delete cancelled");
};

const createNewFile = (): void => {
  openInNewTab("# New Document\n\n", "untitled.md", "", false);
  updateStatus("New file created");
};

const triggerOpenFile = (): void => {
  fileInput.value?.click();
};

const handleFileSelected = async (event: Event): Promise<void> => {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;

  const text = await file.text();
  openInNewTab(text, file.name, "", false);
  updateStatus(`Opened ${file.name} (Browser fallback)`);
  target.value = "";
};

const downloadFile = (name: string, data: string, type: string): void => {
  const blob = new Blob([data], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const saveFallback = (): void => {
  downloadFile(fileName.value, content.value, "text/markdown;charset=utf-8");
  dirty.value = false;
  markSavedNow();
  updateStatus(`Downloaded ${fileName.value} (Browser fallback)`);
};

const saveAsFallback = (): void => {
  const nextName = window.prompt("Save as filename", fileName.value);
  if (!nextName) {
    updateStatus("Save As cancelled");
    return;
  }

  const normalized = /\.(md|markdown|txt)$/i.test(nextName) ? nextName : `${nextName}.md`;
  fileName.value = normalized;
  saveFallback();
};

const openFile = async (): Promise<void> => {
  if (isWailsRuntime()) {
    try {
      const result = await OpenMarkdownFile();
      if (!result) {
        updateStatus("Open cancelled");
        return;
      }
      openInNewTab(result.content, result.name, result.path, false);
      void addRecentFile(result.path, result.name);
      updateStatus(`Opened ${result.name}`);
      return;
    } catch (error) {
      updateStatus(`Open failed: ${String(error)}`);
      return;
    }
  }

  triggerOpenFile();
};

const openRecentFile = async (item: RecentFile): Promise<void> => {
  if (!isWailsRuntime()) {
    updateStatus("Recent-file open only works in desktop mode");
    return;
  }

  try {
    const result = await OpenMarkdownFileAtPath(item.path);
    if (!result) {
      await removeRecentFileByPath(item.path, item.name, true);
      updateStatus(`Recent file not found and removed: ${item.name}`);
      return;
    }
    openInNewTab(result.content, result.name, result.path, false);
    void addRecentFile(result.path, result.name);
    updateStatus(`Opened ${result.name}`);
  } catch {
    await removeRecentFileByPath(item.path, item.name, true);
    updateStatus(`Failed to open; removed stale recent: ${item.name}`);
  }
};

const isMarkdownPath = (path: string): boolean => /\.(md|markdown|txt)$/i.test(path);

const openDroppedPath = async (path: string): Promise<void> => {
  if (!isWailsRuntime()) return;
  try {
    const result = await OpenMarkdownFileAtPath(path);
    if (!result) {
      updateStatus("Dropped file not found");
      return;
    }
    openInNewTab(result.content, result.name, result.path, false);
    void addRecentFile(result.path, result.name);
    updateStatus(`Opened ${result.name} (drop)`);
  } catch (error) {
    updateStatus(`Open dropped file failed: ${String(error)}`);
  }
};

const handleAppFileDrop = (paths: string[]): void => {
  if (!Array.isArray(paths) || paths.length === 0) return;
  const markdownPath = paths.find((path) => isMarkdownPath(path));
  if (!markdownPath) {
    updateStatus("Dropped file ignored: only .md/.markdown/.txt are supported");
    return;
  }
  void openDroppedPath(markdownPath);
};

const saveCurrentFile = async (): Promise<void> => {
  if (isWailsRuntime()) {
    try {
      autosaveState.value = "saving";
      autosaveErrorText.value = "";
      const result = await SaveMarkdownFile({ path: filePath.value, fileName: fileName.value, content: content.value });
      if (!result) {
        autosaveState.value = dirty.value ? "pending" : "idle";
        updateStatus("Save cancelled");
        return;
      }
      filePath.value = result.path;
      fileName.value = result.name;
      dirty.value = false;
      markSavedNow();
      void addRecentFile(result.path, result.name);
      updateStatus(`Saved ${result.name}`);
      return;
    } catch (error) {
      autosaveState.value = "error";
      recordAutosaveError("save", normalizeErrorText(error));
      updateStatus(`Save failed: ${autosaveErrorText.value}`);
      return;
    }
  }

  saveFallback();
};

const saveAsFile = async (): Promise<void> => {
  if (isWailsRuntime()) {
    try {
      autosaveState.value = "saving";
      autosaveErrorText.value = "";
      const result = await SaveMarkdownFile({ path: "", fileName: fileName.value, content: content.value });
      if (!result) {
        autosaveState.value = dirty.value ? "pending" : "idle";
        updateStatus("Save As cancelled");
        return;
      }
      filePath.value = result.path;
      fileName.value = result.name;
      dirty.value = false;
      markSavedNow();
      void addRecentFile(result.path, result.name);
      updateStatus(`Saved As ${result.name}`);
      return;
    } catch (error) {
      autosaveState.value = "error";
      recordAutosaveError("saveAs", normalizeErrorText(error));
      updateStatus(`Save As failed: ${autosaveErrorText.value}`);
      return;
    }
  }

  saveAsFallback();
};

const exportHtml = (): void => {
  const html = `<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>${escapeHtml(fileName.value)}</title>\n</head>\n<body>\n${renderedHtml.value}\n</body>\n</html>`;
  const exportName = fileName.value.replace(/\.(md|markdown|txt)$/i, "") || "document";
  downloadFile(`${exportName}.html`, html, "text/html;charset=utf-8");
  updateStatus(`Exported ${exportName}.html`);
};

const exportPdf = (): void => {
  if (isWailsRuntime()) {
    void (async () => {
      try {
        const result = await ExportMarkdownAsPDF({
          path: "",
          fileName: fileName.value.replace(/\.(md|markdown|txt)$/i, ".pdf"),
          content: content.value,
          documentPath: filePath.value,
        });
        if (!result) {
          updateStatus("Export PDF cancelled");
          return;
        }
        updateStatus(`Exported ${result.name}`);
      } catch (error) {
        updateStatus(`Export PDF failed: ${String(error)}`);
      }
    })();
    return;
  }

  const exportName = fileName.value.replace(/\.(md|markdown|txt)$/i, "") || "document";
  const printWindow = window.open("", "_blank", "noopener,noreferrer,width=980,height=720");
  if (!printWindow) {
    updateStatus("Export PDF blocked by popup policy");
    return;
  }

  const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${escapeHtml(exportName)}</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;padding:24px;line-height:1.6;color:#111}
pre{background:#f3f5f9;padding:10px;border-radius:8px;overflow:auto}
code{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace}
</style>
</head>
<body>${renderedHtml.value}</body>
</html>`;
  printWindow.document.open();
  printWindow.document.write(html);
  printWindow.document.close();
  printWindow.focus();
  printWindow.print();
  printWindow.close();
  updateStatus(`Export PDF started for ${exportName}`);
};

const undoEdit = (): void => {
  if (!editorView) return;
  undo(editorView);
  editorView.focus();
};

const redoEdit = (): void => {
  if (!editorView) return;
  redo(editorView);
  editorView.focus();
};

const openReplacePanel = async (): Promise<void> => {
  showReplacePanel.value = true;
  await nextTick();
  searchInput.value?.focus();
};

const closeReplacePanel = (): void => {
  showReplacePanel.value = false;
  editorView?.focus();
};

const normalizeForMatch = (value: string): string => (matchCase.value ? value : value.toLowerCase());

const findNext = (): void => {
  if (!editorView) return;
  const query = searchQuery.value;
  if (!query) {
    updateStatus("Enter search text");
    return;
  }

  const doc = editorView.state.doc.toString();
  const normalizedDoc = normalizeForMatch(doc);
  const normalizedQuery = normalizeForMatch(query);
  const start = editorView.state.selection.main.to;
  let index = normalizedDoc.indexOf(normalizedQuery, start);
  if (index < 0 && start > 0) index = normalizedDoc.indexOf(normalizedQuery, 0);

  if (index < 0) {
    updateStatus(`No matches for "${query}"`);
    return;
  }

  const to = index + query.length;
  editorView.dispatch({
    selection: EditorSelection.range(index, to),
    effects: EditorView.scrollIntoView(index, { y: "center" }),
  });
  editorView.focus();
  updateStatus(`Match found at ${index}`);
};

const replaceCurrent = (): void => {
  if (!editorView) return;
  const query = searchQuery.value;
  if (!query) {
    updateStatus("Enter search text");
    return;
  }

  const sel = editorView.state.selection.main;
  const selectedText = editorView.state.doc.sliceString(sel.from, sel.to);
  if (normalizeForMatch(selectedText) !== normalizeForMatch(query)) {
    findNext();
    return;
  }

  editorView.dispatch({
    changes: { from: sel.from, to: sel.to, insert: replaceQuery.value },
    selection: EditorSelection.range(sel.from, sel.from + replaceQuery.value.length),
  });
  updateStatus("Replaced current match");
  findNext();
};

const replaceAll = (): void => {
  const query = searchQuery.value;
  if (!query) {
    updateStatus("Enter search text");
    return;
  }

  const regex = new RegExp(escapeRegExp(query), matchCase.value ? "g" : "gi");
  const matches = content.value.match(regex);
  const count = matches?.length ?? 0;
  if (count === 0) {
    updateStatus(`No matches for "${query}"`);
    return;
  }

  const nextDoc = content.value.replace(regex, replaceQuery.value);
  setDocument(nextDoc, fileName.value, filePath.value);
  dirty.value = true;
  scheduleAutosave();
  updateStatus(`Replaced ${count} occurrence${count > 1 ? "s" : ""}`);
};

const toggleTheme = (): void => {
  isDarkTheme.value = !isDarkTheme.value;
  if (editorView) {
    editorView.dispatch({ effects: themeCompartment.reconfigure(currentEditorTheme()) });
  }
  updateStatus(`Theme: ${isDarkTheme.value ? "Dark" : "Light"}`);
};

const toggleLanguage = (): void => {
  uiLanguage.value = uiLanguage.value === "zh" ? "en" : "zh";
  updateStatus(uiLanguage.value === "zh" ? "语言：中文" : "Language: English");
};

const toggleSidebar = (): void => {
  showSidebar.value = !showSidebar.value;
};

const toggleZenMode = (): void => {
  isZenMode.value = !isZenMode.value;
  updateStatus(`Zen mode: ${isZenMode.value ? "on" : "off"}`);
};

const toggleScrollSync = (): void => {
  isScrollSyncEnabled.value = !isScrollSyncEnabled.value;
  updateStatus(`Scroll sync: ${isScrollSyncEnabled.value ? "on" : "off"}`);
};

const toggleLineNumbers = (): void => {
  showLineNumbers.value = !showLineNumbers.value;
  if (editorView) {
    editorView.dispatch({
      effects: lineNumberCompartment.reconfigure(showLineNumbers.value ? lineNumbers() : []),
    });
  }
  updateStatus(`Line numbers: ${showLineNumbers.value ? "on" : "off"}`);
};

const toggleWrapLines = (): void => {
  wrapLines.value = !wrapLines.value;
  if (editorView) {
    editorView.dispatch({
      effects: wrapCompartment.reconfigure(wrapLines.value ? EditorView.lineWrapping : []),
    });
  }
  updateStatus(`Wrap lines: ${wrapLines.value ? "on" : "off"}`);
};

const setEditorFontSize = (nextSize: number): void => {
  const bounded = Math.max(12, Math.min(22, nextSize));
  editorFontSize.value = bounded;
  if (editorView) {
    editorView.dispatch({
      effects: fontSizeCompartment.reconfigure(editorFontSizeTheme(bounded)),
    });
  }
  updateStatus(`Editor font: ${bounded}px`);
};

const setEditorFontFamily = (nextFamily: string): void => {
  const family = EDITOR_FONT_FAMILY_OPTIONS.some((item) => item.value === nextFamily)
    ? nextFamily
    : EDITOR_FONT_FAMILY_OPTIONS[0].value;
  editorFontFamily.value = family;
  if (editorView) {
    editorView.dispatch({
      effects: fontFamilyCompartment.reconfigure(editorFontFamilyTheme(family)),
    });
  }
};

const increaseEditorFontSize = (): void => {
  setEditorFontSize(editorFontSize.value + 1);
};

const decreaseEditorFontSize = (): void => {
  setEditorFontSize(editorFontSize.value - 1);
};

const resetEditorFontSize = (): void => {
  setEditorFontSize(14);
};

const toggleStatusbar = (): void => {
  showStatusbar.value = !showStatusbar.value;
  updateStatus(`Statusbar: ${showStatusbar.value ? "on" : "off"}`);
};

const toggleAutosave = (): void => {
  isAutosaveEnabled.value = !isAutosaveEnabled.value;
  if (!isAutosaveEnabled.value) {
    autosaveState.value = "idle";
    autosaveErrorText.value = "";
  } else if (dirty.value) {
    autosaveState.value = "pending";
  }
  updateStatus(`Autosave: ${isAutosaveEnabled.value ? "on" : "off"}`);
};

const setAutosaveInterval = (nextMs: number): void => {
  if (
    !AUTOSAVE_INTERVAL_OPTIONS.includes(nextMs as (typeof AUTOSAVE_INTERVAL_OPTIONS)[number]) ||
    nextMs === autosaveIntervalMs.value
  ) {
    return;
  }
  autosaveIntervalMs.value = nextMs;
  updateStatus(`Autosave interval: ${autosaveIntervalLabel.value}`);
  if (dirty.value) scheduleAutosave();
};

const cycleAutosaveInterval = (): void => {
  const idx = AUTOSAVE_INTERVAL_OPTIONS.indexOf(autosaveIntervalMs.value as (typeof AUTOSAVE_INTERVAL_OPTIONS)[number]);
  const nextIdx = idx < 0 ? 0 : (idx + 1) % AUTOSAVE_INTERVAL_OPTIONS.length;
  setAutosaveInterval(AUTOSAVE_INTERVAL_OPTIONS[nextIdx]);
};

const onAutosaveIntervalChange = (event: Event): void => {
  const target = event.target as HTMLSelectElement | null;
  if (!target) return;
  const nextMs = Number.parseInt(target.value, 10);
  setAutosaveInterval(nextMs);
};

const retryAutosave = async (): Promise<void> => {
  if (autosaveState.value === "saving") return;
  if (!dirty.value) {
    updateStatus("No pending changes to save");
    return;
  }
  if (!isAutosaveEnabled.value) {
    updateStatus("Autosave is off");
    return;
  }
  await saveCurrentFile();
};

const showAutosaveError = (): void => {
  if (!autosaveErrorText.value && autosaveErrorHistory.value.length === 0) {
    showAutosaveErrorPanel.value = false;
    updateStatus("No autosave error");
    return;
  }
  if (!autosaveErrorActiveId.value && autosaveErrorHistory.value.length > 0) {
    autosaveErrorActiveId.value = autosaveErrorHistory.value[0].id;
  }
  showAutosaveErrorPanel.value = true;
  updateStatus("Opened autosave error details");
};

const copyAutosaveError = async (): Promise<void> => {
  const detail = activeAutosaveErrorDetail.value;
  if (!detail || detail === "No autosave error") {
    updateStatus("No autosave error to copy");
    return;
  }
  try {
    await navigator.clipboard.writeText(detail);
    updateStatus("Autosave error copied");
  } catch {
    updateStatus("Copy failed: clipboard unavailable");
  }
};

const formatNowForFileName = (): string => {
  const d = new Date();
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mi = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${yyyy}${mm}${dd}-${hh}${mi}${ss}`;
};

const buildAutosaveErrorLog = (): string => {
  const lines: string[] = [];
  lines.push(`# nmd autosave error log`);
  lines.push(`generated_at=${new Date().toLocaleString()}`);
  lines.push(`document=${filePath.value || fileName.value || "untitled.md"}`);
  lines.push("");

  if (autosaveErrorHistory.value.length === 0) {
    lines.push("No autosave error history.");
    return lines.join("\n");
  }

  autosaveErrorHistory.value.forEach((item, idx) => {
    lines.push(`[${idx + 1}] at=${item.at} source=${item.source}`);
    lines.push(item.message);
    lines.push("");
  });

  return lines.join("\n");
};

const exportAutosaveErrorLog = async (): Promise<void> => {
  const hasAnyError = autosaveErrorHistory.value.length > 0 || Boolean(autosaveErrorText.value.trim());
  if (!hasAnyError) {
    updateStatus("No autosave error history to export");
    return;
  }

  const logText = buildAutosaveErrorLog();
  const exportName = `nmd-autosave-errors-${formatNowForFileName()}.log`;

  if (isWailsRuntime()) {
    try {
      const result = await SaveMarkdownFile({
        path: "",
        fileName: exportName,
        content: logText,
      });
      if (!result) {
        updateStatus("Export error log cancelled");
        return;
      }
      updateStatus(`Exported autosave error log: ${result.path}`);
      return;
    } catch {
      // Fall back to browser-style download.
    }
  }

  downloadFile(exportName, logText, "text/plain;charset=utf-8");
  updateStatus(`Exported autosave error log: ${exportName}`);
};

const pushAutosaveDeletedSnapshot = (items: AutosaveErrorEntry[]): void => {
  if (items.length === 0) return;
  autosaveErrorUndoStack.value = [[...items], ...autosaveErrorUndoStack.value].slice(0, 20);
  autosaveErrorRedoStack.value = [];
};

const clearAutosaveError = (): void => {
  pushAutosaveDeletedSnapshot(autosaveErrorHistory.value);
  autosaveErrorText.value = "";
  autosaveErrorHistory.value = [];
  autosaveErrorActiveId.value = "";
  autosaveErrorSourceFilter.value = "all";
  autosaveErrorQuery.value = "";
  autosaveErrorSortOrder.value = "desc";
  autosaveErrorSelectedIds.value = [];
  showAutosaveErrorPanel.value = false;
  if (autosaveState.value === "error") {
    autosaveState.value = dirty.value && isAutosaveEnabled.value ? "pending" : "idle";
  }
  updateStatus("Autosave error cleared");
};

const undoAutosaveErrorDelete = (): void => {
  const snapshot = autosaveErrorUndoStack.value[0] || [];
  if (snapshot.length === 0) {
    updateStatus("No deleted autosave errors to restore");
    return;
  }
  autosaveErrorUndoStack.value = autosaveErrorUndoStack.value.slice(1);
  const existing = new Set(autosaveErrorHistory.value.map((item) => item.id));
  const restored = snapshot.filter((item) => !existing.has(item.id));
  if (restored.length === 0) {
    updateStatus("No deleted autosave errors to restore");
    return;
  }
  autosaveErrorHistory.value = [...restored, ...autosaveErrorHistory.value].slice(0, 20);
  autosaveErrorRedoStack.value = [[...restored], ...autosaveErrorRedoStack.value].slice(0, 20);
  autosaveErrorActiveId.value = autosaveErrorHistory.value[0]?.id || "";
  if (!autosaveErrorText.value && autosaveErrorHistory.value.length > 0) {
    autosaveErrorText.value = autosaveErrorHistory.value[0].message;
  }
  updateStatus(`Restored ${restored.length} autosave error item${restored.length > 1 ? "s" : ""}`);
};

const redoAutosaveErrorDelete = (): void => {
  const snapshot = autosaveErrorRedoStack.value[0] || [];
  if (snapshot.length === 0) {
    updateStatus("No restored autosave errors to delete again");
    return;
  }
  autosaveErrorRedoStack.value = autosaveErrorRedoStack.value.slice(1);
  const ids = new Set(snapshot.map((item) => item.id));
  const removed = autosaveErrorHistory.value.filter((item) => ids.has(item.id));
  if (removed.length === 0) {
    updateStatus("No restored autosave errors to delete again");
    return;
  }
  autosaveErrorHistory.value = autosaveErrorHistory.value.filter((item) => !ids.has(item.id));
  autosaveErrorUndoStack.value = [[...removed], ...autosaveErrorUndoStack.value].slice(0, 20);
  autosaveErrorSelectedIds.value = autosaveErrorSelectedIds.value.filter((id) => !ids.has(id));

  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";
  if (autosaveErrorHistory.value.length === 0) {
    autosaveErrorText.value = "";
    if (autosaveState.value === "error") {
      autosaveState.value = dirty.value && isAutosaveEnabled.value ? "pending" : "idle";
    }
  } else if (!autosaveErrorHistory.value.some((item) => item.message === autosaveErrorText.value)) {
    autosaveErrorText.value = autosaveErrorHistory.value[0].message;
  }
  updateStatus(`Deleted ${removed.length} autosave error item${removed.length > 1 ? "s" : ""} again`);
};

const requestClearAutosaveError = (): void => {
  if (autosaveErrorHistory.value.length === 0 && !autosaveErrorText.value) {
    updateStatus("No autosave error to clear");
    return;
  }
  requestDeleteConfirm("Clear all autosave errors and history?", clearAutosaveError);
};

const selectAutosaveError = (id: string): void => {
  autosaveErrorActiveId.value = id;
};

const toggleAutosaveErrorSelected = (id: string): void => {
  const set = new Set(autosaveErrorSelectedIds.value);
  if (set.has(id)) set.delete(id);
  else set.add(id);
  autosaveErrorSelectedIds.value = Array.from(set);
};

const toggleSelectAllFilteredAutosaveErrors = (): void => {
  if (filteredAutosaveErrorHistory.value.length === 0) return;
  if (allFilteredAutosaveErrorsSelected.value) {
    const visible = new Set(filteredAutosaveErrorHistory.value.map((item) => item.id));
    autosaveErrorSelectedIds.value = autosaveErrorSelectedIds.value.filter((id) => !visible.has(id));
    return;
  }
  const set = new Set(autosaveErrorSelectedIds.value);
  filteredAutosaveErrorHistory.value.forEach((item) => set.add(item.id));
  autosaveErrorSelectedIds.value = Array.from(set);
};

const removeAutosaveError = (id: string): void => {
  const removed = autosaveErrorHistory.value.filter((item) => item.id === id);
  if (removed.length === 0) return;
  pushAutosaveDeletedSnapshot(removed);
  autosaveErrorHistory.value = autosaveErrorHistory.value.filter((item) => item.id !== id);
  autosaveErrorSelectedIds.value = autosaveErrorSelectedIds.value.filter((v) => v !== id);

  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";

  if (autosaveErrorHistory.value.length === 0) {
    autosaveErrorText.value = "";
    if (autosaveState.value === "error") {
      autosaveState.value = dirty.value && isAutosaveEnabled.value ? "pending" : "idle";
    }
  } else if (!autosaveErrorHistory.value.some((item) => item.message === autosaveErrorText.value)) {
    autosaveErrorText.value = autosaveErrorHistory.value[0].message;
  }

  updateStatus("Autosave error removed");
};

const requestRemoveAutosaveError = (id: string): void => {
  requestDeleteConfirm("Delete this autosave error item?", () => removeAutosaveError(id));
};

const removeSelectedAutosaveErrors = (): void => {
  if (autosaveErrorSelectedIds.value.length === 0) {
    updateStatus("No selected autosave errors");
    return;
  }
  const selected = new Set(autosaveErrorSelectedIds.value);
  pushAutosaveDeletedSnapshot(autosaveErrorHistory.value.filter((item) => selected.has(item.id)));
  autosaveErrorHistory.value = autosaveErrorHistory.value.filter((item) => !selected.has(item.id));
  autosaveErrorSelectedIds.value = [];

  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";

  if (autosaveErrorHistory.value.length === 0) {
    autosaveErrorText.value = "";
    if (autosaveState.value === "error") {
      autosaveState.value = dirty.value && isAutosaveEnabled.value ? "pending" : "idle";
    }
  } else if (!autosaveErrorHistory.value.some((item) => item.message === autosaveErrorText.value)) {
    autosaveErrorText.value = autosaveErrorHistory.value[0].message;
  }

  updateStatus("Selected autosave errors removed");
};

const requestRemoveSelectedAutosaveErrors = (): void => {
  if (autosaveErrorSelectedIds.value.length === 0) {
    updateStatus("No selected autosave errors");
    return;
  }
  requestDeleteConfirm(`Delete ${autosaveErrorSelectedIds.value.length} selected autosave errors?`, removeSelectedAutosaveErrors);
};

const removeFilteredAutosaveErrors = (): void => {
  if (filteredAutosaveErrorHistory.value.length === 0) {
    updateStatus("No filtered autosave errors");
    return;
  }
  const visible = new Set(filteredAutosaveErrorHistory.value.map((item) => item.id));
  pushAutosaveDeletedSnapshot(autosaveErrorHistory.value.filter((item) => visible.has(item.id)));
  autosaveErrorHistory.value = autosaveErrorHistory.value.filter((item) => !visible.has(item.id));
  autosaveErrorSelectedIds.value = autosaveErrorSelectedIds.value.filter((id) => !visible.has(id));

  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";

  if (autosaveErrorHistory.value.length === 0) {
    autosaveErrorText.value = "";
    if (autosaveState.value === "error") {
      autosaveState.value = dirty.value && isAutosaveEnabled.value ? "pending" : "idle";
    }
  } else if (!autosaveErrorHistory.value.some((item) => item.message === autosaveErrorText.value)) {
    autosaveErrorText.value = autosaveErrorHistory.value[0].message;
  }

  updateStatus("Filtered autosave errors removed");
};

const requestRemoveFilteredAutosaveErrors = (): void => {
  const count = filteredAutosaveErrorHistory.value.length;
  if (count === 0) {
    updateStatus("No filtered autosave errors");
    return;
  }
  requestDeleteConfirm(`Delete all ${count} filtered autosave errors?`, removeFilteredAutosaveErrors);
};

const setAutosaveErrorSourceFilter = (nextFilter: AutosaveErrorSourceFilter): void => {
  autosaveErrorSourceFilter.value = nextFilter;
  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";
};

const toggleAutosaveErrorSortOrder = (): void => {
  autosaveErrorSortOrder.value = autosaveErrorSortOrder.value === "desc" ? "asc" : "desc";
  const first = filteredAutosaveErrorHistory.value[0];
  autosaveErrorActiveId.value = first ? first.id : "";
};

const refreshWindowMaximisedState = async (): Promise<void> => {
  if (!isWailsRuntime()) {
    isWindowMaximised.value = false;
    return;
  }
  try {
    isWindowMaximised.value = await WindowIsMaximised();
  } catch {
    isWindowMaximised.value = false;
  }
};

const toggleWindowMaximise = async (): Promise<void> => {
  if (!isWailsRuntime()) {
    updateStatus("Window maximize is available in desktop mode only");
    return;
  }
  try {
    WindowToggleMaximise();
    setTimeout(() => {
      void refreshWindowMaximisedState();
    }, 120);
  } catch (error) {
    updateStatus(`Toggle maximize failed: ${String(error)}`);
  }
};

const setViewMode = (mode: "split" | "edit" | "preview"): void => {
  viewMode.value = mode;
};

const runCommand = (cmd: Command): void => {
  if (cmd === "new") return createNewFile();
  if (cmd === "open") return void openFile();
  if (cmd === "save") return void saveCurrentFile();
  if (cmd === "saveAs") return void saveAsFile();
  if (cmd === "export") return exportHtml();
  if (cmd === "exportPdf") return exportPdf();
  if (cmd === "undo") return undoEdit();
  if (cmd === "redo") return redoEdit();
  if (cmd === "find") return findNext();
  if (cmd === "gotoLine") return goToLine();
  if (cmd === "replace") return void openReplacePanel();
  if (cmd === "replaceAll") return replaceAll();
  if (cmd === "fmtBold") return applyWrap("**", "**");
  if (cmd === "fmtItalic") return applyWrap("*", "*");
  if (cmd === "fmtCode") return applyCodeBlock();
  if (cmd === "fmtH1") return applyHeading(1);
  if (cmd === "fmtH2") return applyHeading(2);
  if (cmd === "fmtQuote") return applyLinePrefix("> ");
  if (cmd === "fmtBullet") return applyLinePrefix("- ");
  if (cmd === "toggleLanguage") return toggleLanguage();
  if (cmd === "toggleSidebar") return toggleSidebar();
  if (cmd === "toggleZen") return toggleZenMode();
  if (cmd === "toggleScrollSync") return toggleScrollSync();
  if (cmd === "toggleLineNumbers") return toggleLineNumbers();
  if (cmd === "toggleWrapLines") return toggleWrapLines();
  if (cmd === "toggleStatusbar") return toggleStatusbar();
  if (cmd === "toggleAutosave") return toggleAutosave();
  if (cmd === "cycleAutosaveInterval") return cycleAutosaveInterval();
  if (cmd === "retryAutosave") return void retryAutosave();
  if (cmd === "showAutosaveError") return showAutosaveError();
  if (cmd === "copyAutosaveError") return void copyAutosaveError();
  if (cmd === "exportAutosaveErrorLog") return void exportAutosaveErrorLog();
  if (cmd === "clearAutosaveError") return requestClearAutosaveError();
  if (cmd === "undoAutosaveErrorDelete") return undoAutosaveErrorDelete();
  if (cmd === "redoAutosaveErrorDelete") return redoAutosaveErrorDelete();
  if (cmd === "fontSmaller") return decreaseEditorFontSize();
  if (cmd === "fontLarger") return increaseEditorFontSize();
  if (cmd === "fontReset") return resetEditorFontSize();
  if (cmd === "resetLayout") return resetLayoutSizes();
  if (cmd === "viewSplit") return setViewMode("split");
  if (cmd === "viewEditOnly") return setViewMode("edit");
  if (cmd === "viewPreviewOnly") return setViewMode("preview");
  if (cmd === "toggleMaximise") return void toggleWindowMaximise();
  if (cmd === "help") {
    showHelpPanel.value = !showHelpPanel.value;
    return;
  }
  if (cmd === "settings") {
    showSettingsPanel.value = !showSettingsPanel.value;
    return;
  }
  if (cmd === "showUsage") {
    showUsagePanel.value = true;
    return;
  }
  if (cmd === "toggleTheme") return toggleTheme();
  if (cmd === "palette") {
    showCommandPalette.value = !showCommandPalette.value;
    if (showCommandPalette.value) {
      paletteQuery.value = "";
      paletteActiveIndex.value = 0;
    }
  }
};

const onKeydown = (event: KeyboardEvent): void => {
  const rawKey = event.key.toLowerCase();
  if (showCommandPalette.value) {
    if (rawKey === "arrowdown") {
      event.preventDefault();
      if (filteredPaletteCommands.value.length > 0) {
        paletteActiveIndex.value = (paletteActiveIndex.value + 1) % filteredPaletteCommands.value.length;
      }
      return;
    }
    if (rawKey === "arrowup") {
      event.preventDefault();
      if (filteredPaletteCommands.value.length > 0) {
        const next = paletteActiveIndex.value - 1;
        paletteActiveIndex.value = next < 0 ? filteredPaletteCommands.value.length - 1 : next;
      }
      return;
    }
    if (rawKey === "enter") {
      event.preventDefault();
      executePaletteAt(paletteActiveIndex.value);
      return;
    }
  }
  if (showHelpPanel.value) {
    if (rawKey === "arrowdown") {
      event.preventDefault();
      if (visibleHelpShortcutItems.value.length > 0) {
        helpActiveIndex.value = (helpActiveIndex.value + 1) % visibleHelpShortcutItems.value.length;
      }
      return;
    }
    if (rawKey === "arrowup") {
      event.preventDefault();
      if (visibleHelpShortcutItems.value.length > 0) {
        const next = helpActiveIndex.value - 1;
        helpActiveIndex.value = next < 0 ? visibleHelpShortcutItems.value.length - 1 : next;
      }
      return;
    }
    if (rawKey === "enter") {
      event.preventDefault();
      const target = visibleHelpShortcutItems.value[helpActiveIndex.value];
      if (target) {
        runCommand(target.item.id);
      }
      return;
    }
  }
  if (rawKey === "escape") {
    if (isZenMode.value) {
      isZenMode.value = false;
      event.preventDefault();
      return;
    }
    if (showDeleteConfirm.value) {
      cancelDeleteAction();
      event.preventDefault();
      return;
    }
    if (showUnsavedConfirm.value) {
      cancelDiscard();
      event.preventDefault();
      return;
    }
    if (showHelpPanel.value) {
      showHelpPanel.value = false;
      event.preventDefault();
      return;
    }
    if (showSettingsPanel.value) {
      showSettingsPanel.value = false;
      event.preventDefault();
      return;
    }
    if (showUsagePanel.value) {
      showUsagePanel.value = false;
      event.preventDefault();
      return;
    }
    if (showAutosaveErrorPanel.value) {
      showAutosaveErrorPanel.value = false;
      event.preventDefault();
      return;
    }
    if (showCommandPalette.value) {
      showCommandPalette.value = false;
      event.preventDefault();
      return;
    }
  }
  const target = event.target;
  if (target instanceof HTMLElement && target.classList.contains("settings-shortcut-input")) {
    return;
  }

  const isPrimary = event.metaKey || event.ctrlKey;
  if (!isPrimary) return;

  const key = rawKey;
  if (key === "s" && event.shiftKey) {
    event.preventDefault();
    void saveAsFile();
    return;
  }
  if (key === "s") {
    event.preventDefault();
    void saveCurrentFile();
    return;
  }
  if (key === "o") {
    event.preventDefault();
    void openFile();
    return;
  }
  if (key === "n") {
    event.preventDefault();
    createNewFile();
    return;
  }
  if (matchesShortcutBinding(event, "commandPalette")) {
    event.preventDefault();
    showCommandPalette.value = !showCommandPalette.value;
    if (showCommandPalette.value) {
      paletteQuery.value = "";
      paletteActiveIndex.value = 0;
    }
    return;
  }
  if (matchesShortcutBinding(event, "help")) {
    event.preventDefault();
    showHelpPanel.value = !showHelpPanel.value;
    return;
  }
  if (matchesShortcutBinding(event, "settings")) {
    event.preventDefault();
    showSettingsPanel.value = !showSettingsPanel.value;
    return;
  }
  if (matchesShortcutBinding(event, "usage")) {
    event.preventDefault();
    showUsagePanel.value = true;
    return;
  }
  if (key === "h") {
    event.preventDefault();
    void openReplacePanel();
    return;
  }
  if (key === "l") {
    event.preventDefault();
    goToLine();
    return;
  }
  if (key === "b" && event.shiftKey) {
    event.preventDefault();
    applyWrap("**", "**");
    return;
  }
  if (key === "i" && event.shiftKey) {
    event.preventDefault();
    applyWrap("*", "*");
    return;
  }
  if (key === "`" && event.shiftKey) {
    event.preventDefault();
    applyCodeBlock();
    return;
  }
  if (key === "b") {
    event.preventDefault();
    toggleSidebar();
    return;
  }
  if (key === "z" && event.shiftKey) {
    event.preventDefault();
    redoEdit();
    return;
  }
  if (key === "y") {
    if (!enableRedoWithY.value) return;
    event.preventDefault();
    redoEdit();
    return;
  }
  if (enableZenShortcut.value && matchesShortcutBinding(event, "zen")) {
    event.preventDefault();
    toggleZenMode();
    return;
  }
  if (key === "y" && event.shiftKey) {
    event.preventDefault();
    toggleScrollSync();
    return;
  }
  if (key === "g" && event.shiftKey) {
    event.preventDefault();
    toggleLineNumbers();
    return;
  }
  if (key === "w" && event.shiftKey) {
    event.preventDefault();
    toggleWrapLines();
    return;
  }
  if (key === "u" && event.shiftKey) {
    event.preventDefault();
    toggleStatusbar();
    return;
  }
  if (key === "a" && event.shiftKey) {
    event.preventDefault();
    toggleAutosave();
    return;
  }
  if (key === "t" && event.shiftKey) {
    event.preventDefault();
    cycleAutosaveInterval();
    return;
  }
  if (key === "e" && event.shiftKey) {
    event.preventDefault();
    showAutosaveError();
    return;
  }
  if (key === "r" && event.shiftKey) {
    event.preventDefault();
    undoAutosaveErrorDelete();
    return;
  }
  if (key === "j" && event.shiftKey) {
    event.preventDefault();
    redoAutosaveErrorDelete();
    return;
  }
  if (key === "c" && event.shiftKey) {
    event.preventDefault();
    void copyAutosaveError();
    return;
  }
  if (key === "-" || key === "_") {
    event.preventDefault();
    decreaseEditorFontSize();
    return;
  }
  if (key === "=" || key === "+") {
    event.preventDefault();
    increaseEditorFontSize();
    return;
  }
  if (key === "1" && event.shiftKey) {
    event.preventDefault();
    applyHeading(1);
    return;
  }
  if (key === "2" && event.shiftKey) {
    event.preventDefault();
    applyHeading(2);
    return;
  }
  if (key === "." && event.shiftKey) {
    event.preventDefault();
    applyLinePrefix("> ");
    return;
  }
  if (key === "8" && event.shiftKey) {
    event.preventDefault();
    applyLinePrefix("- ");
    return;
  }
  if (key === "1") {
    event.preventDefault();
    setViewMode("split");
    return;
  }
  if (key === "2") {
    event.preventDefault();
    setViewMode("edit");
    return;
  }
  if (key === "3") {
    event.preventDefault();
    setViewMode("preview");
    return;
  }
  if (key === "0") {
    event.preventDefault();
    resetLayoutSizes();
    return;
  }
  if (key === "m" && event.shiftKey) {
    event.preventDefault();
    void toggleWindowMaximise();
    return;
  }
  if (key === "f") {
    event.preventDefault();
    findNext();
    return;
  }
};

watch(content, () => {
  scheduleAutosave();
  void resolvePreviewImages();
});

watch([content, fileName, filePath, dirty], () => {
  syncActiveTabFromState();
});

watch(paletteQuery, () => {
  paletteActiveIndex.value = 0;
});

watch(filteredPaletteCommands, (items) => {
  if (items.length === 0) {
    paletteActiveIndex.value = 0;
    return;
  }
  if (paletteActiveIndex.value > items.length - 1) {
    paletteActiveIndex.value = items.length - 1;
  }
});

watch(visibleHelpShortcutItems, (items) => {
  if (items.length === 0) {
    helpActiveIndex.value = 0;
    return;
  }
  if (helpActiveIndex.value > items.length - 1) {
    helpActiveIndex.value = items.length - 1;
  }
});

watch(showHelpPanel, (value) => {
  if (!value) return;
  helpQuery.value = "";
  helpActiveIndex.value = 0;
});

watch(filteredAutosaveErrorHistory, (items) => {
  const valid = new Set(items.map((item) => item.id));
  autosaveErrorSelectedIds.value = autosaveErrorSelectedIds.value.filter((id) => valid.has(id));
  if (items.length === 0) {
    autosaveErrorActiveId.value = "";
    return;
  }
  if (!items.some((item) => item.id === autosaveErrorActiveId.value)) {
    autosaveErrorActiveId.value = items[0].id;
  }
});

watch(isDarkTheme, (value) => {
  localStorage.setItem(UI_THEME_KEY, value ? "dark" : "light");
});

watch(uiLanguage, (value) => {
  localStorage.setItem(UI_LANG_KEY, value);
});

watch(enableRedoWithY, (value) => {
  localStorage.setItem(UI_SHORTCUT_REDO_Y_KEY, value ? "1" : "0");
});

watch(enableZenShortcut, (value) => {
  localStorage.setItem(UI_SHORTCUT_ZEN_KEY, value ? "1" : "0");
});

watch(
  shortcutBindings,
  (value) => {
    localStorage.setItem(UI_SHORTCUT_BINDINGS_KEY, JSON.stringify(value));
  },
  { deep: true },
);

watch(showSidebar, (value) => {
  localStorage.setItem(UI_SIDEBAR_KEY, value ? "1" : "0");
});

watch(isZenMode, (value) => {
  localStorage.setItem(UI_ZEN_KEY, value ? "1" : "0");
});

watch(isScrollSyncEnabled, (value) => {
  localStorage.setItem(UI_SCROLL_SYNC_KEY, value ? "1" : "0");
});

watch(showLineNumbers, (value) => {
  localStorage.setItem(UI_LINE_NUMBERS_KEY, value ? "1" : "0");
});

watch(wrapLines, (value) => {
  localStorage.setItem(UI_WRAP_LINES_KEY, value ? "1" : "0");
});

watch(editorFontSize, (value) => {
  localStorage.setItem(UI_EDITOR_FONT_SIZE_KEY, String(value));
});

watch(editorFontFamily, (value) => {
  localStorage.setItem(UI_EDITOR_FONT_FAMILY_KEY, value);
  if (editorView) {
    editorView.dispatch({
      effects: fontFamilyCompartment.reconfigure(editorFontFamilyTheme(value)),
    });
  }
});

watch(showStatusbar, (value) => {
  localStorage.setItem(UI_STATUSBAR_KEY, value ? "1" : "0");
});

watch(isAutosaveEnabled, (value) => {
  localStorage.setItem(UI_AUTOSAVE_KEY, value ? "1" : "0");
});

watch(autosaveIntervalMs, (value) => {
  localStorage.setItem(UI_AUTOSAVE_INTERVAL_KEY, String(value));
});

watch(autosaveErrorHistory, (value) => {
  localStorage.setItem(UI_AUTOSAVE_ERRORS_KEY, JSON.stringify(value));
});

watch(autosaveErrorSourceFilter, (value) => {
  localStorage.setItem(UI_AUTOSAVE_ERROR_SOURCE_KEY, value);
});

watch(autosaveErrorQuery, (value) => {
  localStorage.setItem(UI_AUTOSAVE_ERROR_QUERY_KEY, value);
});

watch(autosaveErrorSortOrder, (value) => {
  localStorage.setItem(UI_AUTOSAVE_ERROR_SORT_KEY, value);
});

watch(
  windowTitle,
  (title) => {
    if (!isWailsRuntime()) return;
    WindowSetTitle(title);
  },
  { immediate: true },
);

watch(viewMode, (value) => {
  localStorage.setItem(UI_VIEWMODE_KEY, value);
});

watch(filePath, (value) => {
  const path = value.trim();
  if (path) {
    localStorage.setItem(LAST_FILE_PATH_KEY, path);
  }
});

watch(
  hasDirtyTabs,
  (value) => {
    if (!isWailsRuntime()) return;
    void SetDirtyState(value).catch(() => {
      // Ignore sync failures; close guard simply won't trigger.
    });
  },
  { immediate: true },
);

watch(splitRatio, (value) => {
  localStorage.setItem(UI_SPLIT_RATIO_KEY, String(value));
});

watch(sidebarWidth, (value) => {
  localStorage.setItem(UI_SIDEBAR_WIDTH_KEY, String(value));
});

watch(collapsedOutlineMap, (value) => {
  localStorage.setItem(UI_OUTLINE_COLLAPSE_KEY, JSON.stringify(value));
});

watch(outlineItems, (items) => {
  const validIds = new Set(items.map((item) => item.id));
  const next: Record<string, boolean> = {};
  for (const [id, collapsed] of Object.entries(collapsedOutlineMap.value)) {
    if (collapsed && validIds.has(id)) next[id] = true;
  }
  collapsedOutlineMap.value = next;
});

watch(activeOutlineId, async (id) => {
  if (!id || !showSidebar.value) return;
  await nextTick();
  const container = outlineListRef.value;
  if (!container) return;
  const target = container.querySelector<HTMLElement>(`[data-outline-id="${id}"]`);
  if (!target) return;
  const containerRect = container.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const isFullyVisible = targetRect.top >= containerRect.top && targetRect.bottom <= containerRect.bottom;
  if (isFullyVisible) return;

  const targetCenter = target.offsetTop + target.offsetHeight / 2;
  const nextScrollTop = Math.max(0, targetCenter - container.clientHeight / 2);
  container.scrollTop = nextScrollTop;
});

watch(filePath, () => {
  imagePreviewMap.value = {};
  void resolvePreviewImages();
});

watch(viewMode, async () => {
  await nextTick();
  if (viewMode.value === "preview") {
    updatePreviewActiveLineFromScroll();
  } else {
    previewActiveLine.value = -1;
  }
  editorView?.requestMeasure();
});

onMounted(() => {
  restoreUIState();
  onWindowResize();
  void loadRecentFiles();
  void refreshWindowMaximisedState();

  if (!editorRoot.value) return;

  const state = EditorState.create({
    doc: content.value,
    extensions: [
      lineNumberCompartment.of(showLineNumbers.value ? lineNumbers() : []),
      history(),
      keymap.of([
        {
          key: "Tab",
          run: indentSelectionWithSpaces,
        },
        {
          key: "Shift-Tab",
          run: outdentSelectionWithSpaces,
        },
        {
          key: "Enter",
          run: continueMarkdownListOnEnter,
        },
      ]),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      markdown(),
      wrapCompartment.of(wrapLines.value ? EditorView.lineWrapping : []),
      themeCompartment.of(currentEditorTheme()),
      fontSizeCompartment.of(editorFontSizeTheme(editorFontSize.value)),
      fontFamilyCompartment.of(editorFontFamilyTheme(editorFontFamily.value)),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          content.value = update.state.doc.toString();
          dirty.value = true;
        }
        if (update.docChanged || update.selectionSet) {
          updateCursorStatus();
        }
      }),
    ],
  });

  editorView = new EditorView({ state, parent: editorRoot.value });
  updateCursorStatus();
  editorScrollElement = editorView.scrollDOM;
  editorScrollElement.addEventListener("scroll", syncPreviewFromEditor, { passive: true });
  editorScrollElement.addEventListener("paste", handleEditorPaste);
  editorScrollElement.addEventListener("dragover", handleEditorDragOver);
  editorScrollElement.addEventListener("drop", handleEditorDrop);

  const hasDraft = restoreDraft();
  if (!hasDraft) {
    void restoreLastFile();
  }
  void resolvePreviewImages();
  if (isWailsRuntime()) {
    OnFileDrop((_x, _y, paths) => {
      handleAppFileDrop(paths);
    }, false);
  }
  window.addEventListener("keydown", onKeydown);
  window.addEventListener("focus", onWindowFocus);
  window.addEventListener("resize", onWindowResize);
  window.addEventListener("beforeunload", onBeforeUnload);
});

onBeforeUnmount(() => {
  if (isWailsRuntime()) {
    OnFileDropOff();
  }
  window.removeEventListener("keydown", onKeydown);
  window.removeEventListener("focus", onWindowFocus);
  window.removeEventListener("resize", onWindowResize);
  window.removeEventListener("beforeunload", onBeforeUnload);
  onSplitDragEnd();
  onSidebarDragEnd();
  if (editorScrollElement) {
    editorScrollElement.removeEventListener("scroll", syncPreviewFromEditor);
    editorScrollElement.removeEventListener("paste", handleEditorPaste);
    editorScrollElement.removeEventListener("dragover", handleEditorDragOver);
    editorScrollElement.removeEventListener("drop", handleEditorDrop);
    editorScrollElement = null;
  }
  if (autosaveTimer) {
    clearTimeout(autosaveTimer);
    autosaveTimer = null;
  }
  if (editorView) {
    editorView.destroy();
    editorView = null;
  }
});
</script>

<template>
  <main class="layout" :class="{ dark: isDarkTheme, zen: isZenMode }" :style="layoutStyle">
    <input ref="fileInput" class="hidden-input" type="file" accept=".md,.markdown,.txt" @change="handleFileSelected" />
    <input ref="settingsImportInput" class="hidden-input" type="file" accept=".json,application/json" @change="onImportSettingsSelected" />

    <header v-if="!isZenMode" class="toolbar">
      <div class="toolbar-left">
        <button class="ghost" @click="runCommand('toggleSidebar')">{{ t('sidebar') }}</button>
        <span class="brand">nmd</span>
        <span class="file-label">{{ fileLabel }}</span>
      </div>
      <div class="toolbar-right">
        <div class="toolbar-group">
          <button class="ghost" :class="{ active: viewMode === 'split' }" @click="setViewMode('split')">{{ t('split') }}</button>
          <button class="ghost" :class="{ active: viewMode === 'edit' }" @click="setViewMode('edit')">{{ t('edit') }}</button>
          <button class="ghost" :class="{ active: viewMode === 'preview' }" @click="setViewMode('preview')">{{ t('preview') }}</button>
          <button class="ghost" @click="runCommand('toggleMaximise')">
            {{ isWindowMaximised ? t('restore') : t('maximize') }}
          </button>
          <button class="ghost" @click="runCommand('new')">{{ t('new') }}</button>
          <button class="ghost" @click="runCommand('open')">{{ t('open') }}</button>
          <button class="ghost" :class="{ active: dirty }" @click="runCommand('save')">{{ t('save') }}</button>
          <button class="ghost" :class="{ active: isAutosaveEnabled }" @click="runCommand('toggleAutosave')">{{ t('autosave') }}</button>
          <select
            class="autosave-select"
            :value="autosaveIntervalMs"
            :disabled="!isAutosaveEnabled"
            :title="t('autosaveIntervalTitle')"
            @change="onAutosaveIntervalChange"
          >
            <option v-for="opt in autosaveIntervalOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
          <button class="ghost" @click="runCommand('replace')">{{ t('findReplace') }}</button>
          <button class="ghost" @click="runCommand('exportPdf')">{{ t('pdf') }}</button>
          <button class="ghost" @click="runCommand('help')">{{ t('shortcuts') }}</button>
          <button class="ghost" @click="runCommand('showUsage')">{{ t('usage') }}</button>
          <button class="ghost" :class="{ active: isScrollSyncEnabled }" @click="runCommand('toggleScrollSync')">{{ t('sync') }}</button>
          <button class="ghost" :class="{ active: showLineNumbers }" @click="runCommand('toggleLineNumbers')">{{ t('lineNo') }}</button>
          <button class="ghost" :class="{ active: wrapLines }" @click="runCommand('toggleWrapLines')">{{ t('wrap') }}</button>
          <button class="ghost" :class="{ active: showStatusbar }" @click="runCommand('toggleStatusbar')">{{ t('bar') }}</button>
          <button class="ghost" @click="runCommand('fontSmaller')">A-</button>
          <button class="ghost" @click="runCommand('fontLarger')">A+</button>
          <button class="ghost" :class="{ active: isZenMode }" @click="runCommand('toggleZen')">{{ t('zen') }}</button>
          <button class="ghost" @click="runCommand('toggleTheme')">{{ isDarkTheme ? t('light') : t('dark') }}</button>
          <button class="ghost" @click="runCommand('settings')">{{ t('settings') }}</button>
          <button class="ghost" @click="runCommand('toggleLanguage')">{{ t('langButton') }}</button>
          <button class="ghost" @click="runCommand('palette')">{{ t('command') }}</button>
        </div>
        <div class="toolbar-divider" />
        <div class="toolbar-group">
          <button class="ghost" @click="runCommand('fmtBold')">{{ t('bold') }}</button>
          <button class="ghost" @click="runCommand('fmtItalic')">{{ t('italic') }}</button>
          <button class="ghost" @click="runCommand('fmtH1')">H1</button>
          <button class="ghost" @click="runCommand('fmtH2')">H2</button>
          <button class="ghost" @click="runCommand('fmtQuote')">{{ t('quote') }}</button>
          <button class="ghost" @click="runCommand('fmtBullet')">{{ t('list') }}</button>
          <button class="ghost" @click="runCommand('fmtCode')">{{ t('code') }}</button>
        </div>
      </div>
    </header>

    <section v-if="!isZenMode" class="tabbar">
      <div v-for="tab in docTabs" :key="tab.id" class="tab-item" :class="{ active: tab.id === activeTabId }">
        <button class="tab-main" @click="activateTab(tab.id)">{{ tab.dirty ? `${tab.name} *` : tab.name }}</button>
        <button class="tab-close-btn" @click.stop="requestCloseTab(tab.id)">x</button>
      </div>
    </section>

    <section v-if="showReplacePanel && !isZenMode" class="replace-panel">
      <input ref="searchInput" v-model="searchQuery" :placeholder="t('find')" />
      <input v-model="replaceQuery" :placeholder="t('replaceWith')" />
      <label class="match-case">
        <input v-model="matchCase" type="checkbox" />
        <span>{{ t('matchCase') }}</span>
      </label>
      <button @click="findNext">{{ t('findNext') }}</button>
      <button @click="replaceCurrent">{{ t('replace') }}</button>
      <button @click="replaceAll">{{ t('replaceAll') }}</button>
      <button @click="closeReplacePanel">{{ t('close') }}</button>
    </section>

    <section ref="workspaceRef" class="workspace" :class="{ 'no-sidebar': !showSidebar || isZenMode }" :style="workspaceStyle">
      <aside v-if="showSidebar && !isZenMode" class="sidebar">
        <div class="sidebar-section-title section-head">
          <span>{{ t('outline') }}</span>
          <span class="section-actions">
            <button
              class="mini-action"
              :disabled="!outlineHasCollapsibleItems"
              :title="t('collapse')"
              @click="collapseAllOutline"
            >
              {{ t('collapse') }}
            </button>
            <button class="mini-action" :title="t('expand')" @click="expandAllOutline">{{ t('expand') }}</button>
          </span>
        </div>
        <div class="sidebar-search-wrap">
          <input v-model="outlineQuery" class="sidebar-search" :placeholder="t('filterHeadings')" />
        </div>
        <div ref="outlineListRef" class="sidebar-list">
          <div
            v-for="item in filteredOutlineItems"
            :key="item.id"
            class="outline-row"
            :class="{ active: item.id === activeOutlineId }"
            :data-outline-id="item.id"
            :style="{ paddingLeft: `${Math.max(6, (item.level - 1) * 14 + 6)}px` }"
          >
            <button
              class="outline-toggle"
              :class="{ placeholder: !item.hasChildren }"
              @click="toggleOutlineCollapse(item)"
            >
              <span v-if="item.hasChildren">{{ item.collapsed ? "▸" : "▾" }}</span>
            </button>
            <button class="sidebar-item outline-title" @click="jumpToOutlineItem(item)">
              {{ item.title }}
            </button>
          </div>
          <p v-if="outlineItems.length === 0" class="sidebar-empty">{{ t('noHeadings') }}</p>
          <p v-else-if="filteredOutlineItems.length === 0" class="sidebar-empty">{{ t('noMatchedHeadings') }}</p>
        </div>

        <div class="sidebar-section-title recent-title section-head">
          <span>{{ t('recentFiles') }}</span>
          <span class="section-actions">
            <button class="mini-action" :disabled="recentFiles.length === 0" :title="t('clear')" @click="clearAllRecentFiles">
              {{ t('clear') }}
            </button>
          </span>
        </div>
        <div class="sidebar-search-wrap">
          <input v-model="recentQuery" class="sidebar-search" :placeholder="t('filterRecent')" />
        </div>
        <div class="sidebar-list recent-list">
          <div
            v-for="item in filteredRecentFiles"
            :key="item.path"
            class="recent-row"
            :class="{ dragging: draggingRecentPath === item.path }"
            draggable="true"
            @dragstart="onRecentDragStart($event, item.path)"
            @dragover="onRecentDragOver"
            @drop="onRecentDrop($event, item.path)"
            @dragend="onRecentDragEnd"
          >
            <button class="sidebar-item recent-open" @click="openRecentFile(item)">
              {{ item.name }}
            </button>
            <button
              class="mini-action"
              :class="{ accent: isRecentPinned(item.path) }"
              :title="t('pinUnpin')"
              @click.stop="togglePinRecentFile(item)"
            >
              {{ isRecentPinned(item.path) ? "★" : "☆" }}
            </button>
            <button class="mini-action danger" :title="t('removeItem')" @click.stop="removeRecentFile(item)">x</button>
          </div>
          <p v-if="recentFiles.length === 0" class="sidebar-empty">{{ t('noRecentFiles') }}</p>
          <p v-else-if="filteredRecentFiles.length === 0" class="sidebar-empty">{{ t('noMatchedFiles') }}</p>
        </div>
      </aside>
      <div
        v-if="showSidebarSplitter && !isZenMode"
        class="sidebar-splitter"
        @mousedown="startSidebarDrag"
        @dblclick="resetSidebarWidth"
      />

      <section ref="docZoneRef" class="doc-zone" :class="`mode-${viewMode}`" :style="docZoneStyle">
        <article class="editor-pane" :class="{ hidden: viewMode === 'preview' }">
          <div ref="editorRoot" class="editor-host" />
        </article>
        <div v-if="viewMode === 'split'" class="splitter" @mousedown="startSplitDrag" @dblclick="resetSplitRatio" />
        <article class="preview-pane" :class="{ hidden: viewMode === 'edit' }">
          <div
            ref="previewRef"
            class="preview markdown-body"
            @scroll.passive="syncEditorFromPreview"
            @click="handlePreviewClick"
            @change="handlePreviewChange"
            @dragover="handlePreviewDragOver"
            @drop="handlePreviewDrop"
            v-html="renderedHtml"
          />
        </article>
      </section>
    </section>

    <footer v-if="!isZenMode && showStatusbar" class="statusbar">
      <span class="status-main">{{ statusText }}</span>
      <span class="status-meta">
        <span>{{ autosaveLabel }}</span>
        <button v-if="autosaveState === 'error'" class="mini-action" @click="runCommand('retryAutosave')">{{ t('retry') }}</button>
        <button
          v-if="autosaveState === 'error' && autosaveErrorText"
          class="status-error"
          :title="autosaveErrorText"
          @click="runCommand('showAutosaveError')"
        >
          {{ autosaveErrorLabel }}
        </button>
        <span>{{ t('auto') }} {{ autosaveIntervalLabel }}</span>
        <span>{{ t('sync') }} {{ isScrollSyncEnabled ? t('on') : t('off') }}</span>
        <span>{{ imageStatus }}</span>
        <span>{{ isDarkTheme ? t('dark') : t('light') }}</span>
        <span>{{ viewMode }}</span>
        <span>{{ filePathLabel }}</span>
        <span>{{ t('utf8') }}</span>
        <span>{{ tf('lineCol', { line: cursorLine, col: cursorCol }) }}</span>
        <span>{{ tf('words', { count: wordCount }) }}</span>
        <span>{{ tf('chars', { count: charCount }) }}</span>
        <span>{{ tf('minRead', { count: readingMinutes }) }}</span>
      </span>
    </footer>

    <section v-if="showCommandPalette" class="palette-mask" @click="showCommandPalette = false">
      <article class="palette" @click.stop>
        <h2>{{ t('commandPalette') }}</h2>
        <input v-model="paletteQuery" class="palette-search" :placeholder="t('typeCommand')" />
        <div class="palette-list">
          <button
            v-for="(cmd, idx) in filteredPaletteCommands"
            :key="cmd.id"
            :class="{ active: idx === paletteActiveIndex }"
            @click="executePaletteAt(idx)"
          >
            {{ cmd.label }} <span class="shortcut">{{ cmd.shortcut }}</span>
          </button>
        </div>
        <p v-if="filteredPaletteCommands.length === 0" class="palette-empty">{{ t('noCommandFound') }}</p>
      </article>
    </section>

    <section v-if="showHelpPanel" class="palette-mask" @click="showHelpPanel = false">
      <article class="palette" @click.stop>
        <h2>{{ t('keyboardShortcuts') }}</h2>
        <input v-model="helpQuery" class="palette-search" :placeholder="t('searchShortcuts')" />
        <div class="help-groups">
          <section v-for="group in filteredHelpShortcutGroups" :key="group.id" class="help-group">
            <button class="help-group-toggle" @click="toggleHelpGroup(group.id)">
              <span>{{ group.title }}</span>
              <span class="shortcut">{{ helpCollapsedGroups[group.id] ? '+' : '-' }} {{ group.items.length }}</span>
            </button>
            <div v-if="!helpCollapsedGroups[group.id]" class="help-list">
              <button
                v-for="item in group.items"
                :key="`${group.id}-${item.id}`"
                :class="{ active: visibleHelpShortcutItems.findIndex((entry) => entry.groupId === group.id && entry.item.id === item.id) === helpActiveIndex }"
                @mouseenter="helpActiveIndex = visibleHelpShortcutItems.findIndex((entry) => entry.groupId === group.id && entry.item.id === item.id)"
                @click="runCommand(item.id)"
              >
                {{ item.label }} <span class="shortcut">{{ item.shortcut }}</span>
              </button>
            </div>
          </section>
          <p v-if="filteredHelpShortcutGroups.length === 0" class="palette-empty">{{ t('noMatchedShortcuts') }}</p>
        </div>
        <button @click="showHelpPanel = false">{{ t('close') }} <span class="shortcut">Esc</span></button>
      </article>
    </section>

    <section v-if="showSettingsPanel" class="palette-mask" @click="showSettingsPanel = false">
      <article class="palette settings-panel" @click.stop>
        <h2>{{ t('globalSettings') }}</h2>
        <div class="settings-grid">
          <section class="settings-section">
            <h3>{{ t('settingsTheme') }}</h3>
            <div class="settings-row">
              <button class="ghost" @click="isDarkTheme = false">{{ t('light') }}</button>
              <button class="ghost" @click="isDarkTheme = true">{{ t('dark') }}</button>
            </div>
            <h3>{{ t('settingsLanguage') }}</h3>
            <div class="settings-row">
              <button class="ghost" @click="uiLanguage = 'zh'">中文</button>
              <button class="ghost" @click="uiLanguage = 'en'">English</button>
            </div>
          </section>

          <section class="settings-section">
            <h3>{{ t('settingsEditor') }}</h3>
            <div class="settings-row">
              <span>{{ t('settingsFontSize') }}: {{ editorFontSize }}px</span>
              <button class="ghost" @click="runCommand('fontSmaller')">A-</button>
              <button class="ghost" @click="runCommand('fontLarger')">A+</button>
            </div>
            <input v-model.number="editorFontSize" type="range" min="12" max="22" step="1" />
            <div class="settings-row">
              <span>{{ t('settingsFontFamily') }}</span>
              <select :value="editorFontFamily" @change="setEditorFontFamily(($event.target as HTMLSelectElement).value)">
                <option v-for="opt in editorFontFamilyOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </div>
          </section>

          <section class="settings-section">
            <h3>{{ t('settingsAppearance') }}</h3>
            <label class="settings-check"><input v-model="showLineNumbers" type="checkbox" />{{ t('settingsLineNumbers') }}</label>
            <label class="settings-check"><input v-model="wrapLines" type="checkbox" />{{ t('settingsWrapLines') }}</label>
            <label class="settings-check"><input v-model="showStatusbar" type="checkbox" />{{ t('settingsStatusbar') }}</label>
            <label class="settings-check"><input v-model="showSidebar" type="checkbox" />{{ t('settingsSidebar') }}</label>
            <label class="settings-check"><input v-model="isScrollSyncEnabled" type="checkbox" />{{ t('settingsScrollSync') }}</label>
          </section>

          <section class="settings-section">
            <h3>{{ t('settingsAutosave') }}</h3>
            <label class="settings-check"><input v-model="isAutosaveEnabled" type="checkbox" />{{ t('settingsAutosaveEnable') }}</label>
            <div class="settings-row">
              <span>{{ t('settingsAutosaveInterval') }}</span>
              <select v-model.number="autosaveIntervalMs" :disabled="!isAutosaveEnabled">
                <option v-for="opt in autosaveIntervalOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </div>
          </section>

          <section class="settings-section">
            <h3>{{ t('settingsShortcut') }}</h3>
            <label class="settings-check"><input v-model="enableRedoWithY" type="checkbox" />{{ t('settingsRedoY') }}</label>
            <label class="settings-check"><input v-model="enableZenShortcut" type="checkbox" />{{ t('settingsZenShortcut') }}</label>
          </section>

          <section class="settings-section">
            <h3>{{ t('settingsKeymap') }}</h3>
            <div class="settings-row">
              <span>{{ t('keyCommandPalette') }}</span>
              <input
                v-model="shortcutBindings.commandPalette"
                class="settings-shortcut-input"
                :class="{ invalid: invalidShortcutKeys.has('commandPalette') }"
                readonly
                :placeholder="t('settingsShortcutPlaceholder')"
                :title="t('settingsShortcutPlaceholder')"
                @keydown="captureShortcutByEvent('commandPalette', $event)"
                @blur="normalizeShortcutInput('commandPalette')"
              />
              <button class="ghost settings-row-action" @click="resetShortcutBinding('commandPalette')">{{ t('settingsUseDefault') }}</button>
            </div>
            <div class="settings-row">
              <span>{{ t('keyHelp') }}</span>
              <input
                v-model="shortcutBindings.help"
                class="settings-shortcut-input"
                :class="{ invalid: invalidShortcutKeys.has('help') }"
                readonly
                :placeholder="t('settingsShortcutPlaceholder')"
                :title="t('settingsShortcutPlaceholder')"
                @keydown="captureShortcutByEvent('help', $event)"
                @blur="normalizeShortcutInput('help')"
              />
              <button class="ghost settings-row-action" @click="resetShortcutBinding('help')">{{ t('settingsUseDefault') }}</button>
            </div>
            <div class="settings-row">
              <span>{{ t('keySettings') }}</span>
              <input
                v-model="shortcutBindings.settings"
                class="settings-shortcut-input"
                :class="{ invalid: invalidShortcutKeys.has('settings') }"
                readonly
                :placeholder="t('settingsShortcutPlaceholder')"
                :title="t('settingsShortcutPlaceholder')"
                @keydown="captureShortcutByEvent('settings', $event)"
                @blur="normalizeShortcutInput('settings')"
              />
              <button class="ghost settings-row-action" @click="resetShortcutBinding('settings')">{{ t('settingsUseDefault') }}</button>
            </div>
            <div class="settings-row">
              <span>{{ t('keyUsage') }}</span>
              <input
                v-model="shortcutBindings.usage"
                class="settings-shortcut-input"
                :class="{ invalid: invalidShortcutKeys.has('usage') }"
                readonly
                :placeholder="t('settingsShortcutPlaceholder')"
                :title="t('settingsShortcutPlaceholder')"
                @keydown="captureShortcutByEvent('usage', $event)"
                @blur="normalizeShortcutInput('usage')"
              />
              <button class="ghost settings-row-action" @click="resetShortcutBinding('usage')">{{ t('settingsUseDefault') }}</button>
            </div>
            <div class="settings-row">
              <span>{{ t('keyZen') }}</span>
              <input
                v-model="shortcutBindings.zen"
                class="settings-shortcut-input"
                :class="{ invalid: invalidShortcutKeys.has('zen') }"
                :disabled="!enableZenShortcut"
                readonly
                :placeholder="t('settingsShortcutPlaceholder')"
                :title="t('settingsShortcutPlaceholder')"
                @keydown="captureShortcutByEvent('zen', $event)"
                @blur="normalizeShortcutInput('zen')"
              />
              <button class="ghost settings-row-action" :disabled="!enableZenShortcut" @click="resetShortcutBinding('zen')">{{ t('settingsUseDefault') }}</button>
            </div>
            <p
              v-for="conflict in shortcutConflicts"
              :key="`${conflict.pattern}-${conflict.keys.join('-')}`"
              class="settings-warning"
            >
              {{ tf('settingsShortcutConflict', { pattern: conflict.pattern, names: conflict.keys.map(getShortcutBindingLabel).join(', ') }) }}
            </p>
            <div class="settings-row">
              <button class="ghost" @click="resetShortcutBindings">{{ t('settingsResetShortcuts') }}</button>
            </div>
            <div class="settings-row">
              <button class="ghost" @click="triggerImportSettings">{{ t('settingsImport') }}</button>
              <button class="ghost" @click="exportSettings">{{ t('settingsExport') }}</button>
            </div>
          </section>
        </div>
        <p class="palette-empty">{{ t('settingsHint') }}</p>
        <button @click="showSettingsPanel = false">{{ t('close') }} <span class="shortcut">Esc</span></button>
      </article>
    </section>

    <section v-if="showUsagePanel" class="palette-mask" @click="showUsagePanel = false">
      <article class="palette usage-panel" @click.stop>
        <h2>{{ t('usage') }}</h2>
        <div class="usage-body markdown-body" v-html="usageHtml" />
        <button @click="showUsagePanel = false">{{ t('close') }} <span class="shortcut">Esc</span></button>
      </article>
    </section>

    <section v-if="showAutosaveErrorPanel" class="palette-mask" @click="showAutosaveErrorPanel = false">
      <article class="palette autosave-error-panel" @click.stop>
        <h2>{{ t('autosaveErrorDetails') }}</h2>
        <div class="autosave-error-filters">
          <button
            v-for="opt in autosaveErrorSourceOptions"
            :key="opt.value"
            class="autosave-filter-btn"
            :class="{ active: autosaveErrorSourceFilter === opt.value }"
            @click="setAutosaveErrorSourceFilter(opt.value)"
          >
            {{ opt.label }}
          </button>
          <button class="autosave-filter-btn" @click="toggleAutosaveErrorSortOrder">
            {{ t('sort') }}: {{ autosaveSortLabel }}
          </button>
          <button class="autosave-filter-btn" @click="toggleSelectAllFilteredAutosaveErrors">
            {{ allFilteredAutosaveErrorsSelected ? t('unselectAll') : t('selectAll') }}
          </button>
          <button
            class="autosave-filter-btn danger"
            :disabled="selectedAutosaveErrorCount === 0"
            @click="requestRemoveSelectedAutosaveErrors"
          >
            {{ tf('deleteSelected', { count: selectedAutosaveErrorCount }) }}
          </button>
          <button
            class="autosave-filter-btn danger"
            :disabled="filteredAutosaveErrorHistory.length === 0"
            @click="requestRemoveFilteredAutosaveErrors"
          >
            {{ tf('deleteFiltered', { count: filteredAutosaveErrorHistory.length }) }}
          </button>
          <input v-model="autosaveErrorQuery" class="autosave-error-search" :placeholder="t('searchError')" />
        </div>
        <div class="autosave-error-grid">
          <div class="autosave-error-list">
            <div
              v-for="item in filteredAutosaveErrorHistory"
              :key="item.id"
              class="autosave-error-row"
              :class="{ active: activeAutosaveError && activeAutosaveError.id === item.id }"
            >
              <input
                class="autosave-error-check"
                type="checkbox"
                :checked="autosaveErrorSelectedIds.includes(item.id)"
                @change="toggleAutosaveErrorSelected(item.id)"
              />
              <button class="autosave-error-item" @click="selectAutosaveError(item.id)">
                <span>{{ item.at }}</span>
                <span>{{ item.source }}</span>
              </button>
              <button class="mini-action danger" :title="t('removeItem')" @click.stop="requestRemoveAutosaveError(item.id)">x</button>
            </div>
            <p v-if="autosaveErrorHistory.length === 0" class="palette-empty">{{ t('noHistory') }}</p>
            <p v-else-if="filteredAutosaveErrorHistory.length === 0" class="palette-empty">{{ t('noMatchedErrors') }}</p>
          </div>
          <pre class="autosave-error-detail">{{ activeAutosaveErrorDetail }}</pre>
        </div>
        <button @click="runCommand('copyAutosaveError')">{{ t('copyError') }} <span class="shortcut">Ctrl/Cmd+Shift+C</span></button>
        <button @click="runCommand('exportAutosaveErrorLog')">{{ t('exportLog') }} <span class="shortcut">-</span></button>
        <button @click="requestClearAutosaveError">{{ t('clearError') }} <span class="shortcut">-</span></button>
        <button :disabled="undoDeletedAutosaveErrorCount === 0" @click="runCommand('undoAutosaveErrorDelete')">
          {{ tf('undoDelete', { count: undoDeletedAutosaveErrorCount }) }} <span class="shortcut">Ctrl/Cmd+Shift+R</span>
        </button>
        <button :disabled="redoDeletedAutosaveErrorCount === 0" @click="runCommand('redoAutosaveErrorDelete')">
          {{ tf('redoDelete', { count: redoDeletedAutosaveErrorCount }) }} <span class="shortcut">Ctrl/Cmd+Shift+J</span>
        </button>
        <button @click="showAutosaveErrorPanel = false">{{ t('close') }} <span class="shortcut">Esc</span></button>
      </article>
    </section>

    <section v-if="showDeleteConfirm" class="palette-mask" @click="cancelDeleteAction">
      <article class="palette" @click.stop>
        <h2>{{ t('confirmDelete') }}</h2>
        <p class="palette-empty">{{ deleteConfirmText }}</p>
        <button class="danger" @click="confirmDeleteAction">{{ t('delete') }}</button>
        <button @click="cancelDeleteAction">{{ t('cancel') }}</button>
      </article>
    </section>

    <section v-if="showUnsavedConfirm" class="palette-mask" @click="cancelDiscard">
      <article class="palette" @click.stop>
        <h2>{{ t('unsavedChanges') }}</h2>
        <p class="palette-empty">{{ t('unsavedConfirm') }}</p>
        <button @click="confirmDiscardAndContinue">{{ t('discardContinue') }}</button>
        <button @click="cancelDiscard">{{ t('cancel') }}</button>
      </article>
    </section>
  </main>
</template>

<style scoped>
.layout {
  height: 100vh;
  height: 100dvh;
  min-height: 100vh;
  min-height: 100dvh;
  padding: 10px;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto auto auto minmax(0, 1fr) auto;
  gap: 10px;
  color: #1f2937;
  background: radial-gradient(circle at 8% 10%, #f8fafc 0, #eef2f7 35%, #e2e8f0 100%);
}

.layout.dark {
  color: #e5e7eb;
  background: radial-gradient(circle at 8% 10%, #172033 0, #111827 45%, #0b1220 100%);
}

.layout.zen {
  grid-template-rows: minmax(0, 1fr);
  padding: 8px;
  gap: 8px;
}

.layout.zen .workspace {
  grid-row: 1;
}

.hidden-input {
  display: none;
}

.tabbar {
  grid-row: 2;
  display: flex;
  gap: 8px;
  overflow: auto;
  padding: 4px 2px;
}

.tab-item {
  display: inline-flex;
  align-items: center;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  min-width: 0;
}

.tab-item.active {
  border-color: #91a5c2;
  background: #e9f0fb;
}

.tab-main {
  border: 0;
  background: transparent;
  color: #1f2937;
  padding: 6px 10px;
  max-width: 220px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
}

.tab-close-btn {
  border: 0;
  border-left: 1px solid #dbe3ef;
  background: transparent;
  color: #64748b;
  padding: 6px 8px;
  cursor: pointer;
}

.layout.dark .tab-item {
  border-color: #42536d;
  background: #1b2537;
}

.layout.dark .tab-item.active {
  border-color: #6e84a5;
  background: #2b3d5a;
}

.layout.dark .tab-main {
  color: #e5e7eb;
}

.layout.dark .tab-close-btn {
  border-left-color: #42536d;
  color: #9fb2cc;
}

.toolbar {
  grid-row: 1;
  position: sticky;
  top: 10px;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 12px;
  border: 1px solid #d9e0ea;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.82);
  backdrop-filter: blur(8px);
}

.layout.dark .toolbar {
  border-color: #2f3d53;
  background: rgba(15, 23, 42, 0.72);
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-divider {
  width: 1px;
  align-self: stretch;
  background: #d2dbe7;
}

.layout.dark .toolbar-divider {
  background: #3b4a60;
}

.brand {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.file-label {
  color: #64748b;
  font-size: 12px;
}

.ghost,
.replace-panel button,
.palette button,
.sidebar-item {
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #1f2937;
  padding: 5px 10px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
}

.autosave-select {
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #1f2937;
  padding: 5px 8px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
}

.autosave-select:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.ghost.active {
  background: #e9f0fb;
  border-color: #91a5c2;
}

.layout.dark .ghost.active {
  background: #2b3d5a;
  border-color: #6e84a5;
}

.layout.dark .ghost,
.layout.dark .replace-panel button,
.layout.dark .palette button,
.layout.dark .sidebar-item {
  border-color: #42536d;
  background: #1b2537;
  color: #e5e7eb;
}

.layout.dark .autosave-select {
  border-color: #42536d;
  background: #1b2537;
  color: #e5e7eb;
}

.ghost:hover,
.replace-panel button:hover,
.palette button:hover,
.sidebar-item:hover {
  background: #eef3f9;
  border-color: #9aa8bc;
}

.autosave-select:hover:not(:disabled) {
  background: #eef3f9;
  border-color: #9aa8bc;
}

.layout.dark .ghost:hover,
.layout.dark .replace-panel button:hover,
.layout.dark .palette button:hover,
.layout.dark .sidebar-item:hover {
  background: #2b3a53;
  border-color: #62748f;
}

.palette button.danger {
  border-color: #ef4444;
  color: #b91c1c;
}

.layout.dark .palette button.danger {
  border-color: #f87171;
  color: #fca5a5;
}

.layout.dark .autosave-select:hover:not(:disabled) {
  background: #2b3a53;
  border-color: #62748f;
}

.replace-panel {
  grid-row: 3;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  padding: 10px;
  border: 1px solid #d6dde8;
  border-radius: 12px;
  background: #f8fbff;
}

.layout.dark .replace-panel {
  border-color: #334155;
  background: #0f172a;
}

.replace-panel input {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 7px 10px;
  min-width: 180px;
  background: #ffffff;
  color: #0f172a;
}

.layout.dark .replace-panel input {
  border-color: #475569;
  background: #111827;
  color: #e5e7eb;
}

.match-case {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.workspace {
  grid-row: 4;
  height: 100%;
  min-height: 0;
  display: grid;
  grid-template-columns: 240px 8px 1fr;
  gap: 10px;
}

.workspace.no-sidebar {
  grid-template-columns: 1fr;
}

.sidebar {
  min-height: 0;
  display: grid;
  grid-template-rows: auto 1fr auto 1fr;
  border: 1px solid #d9e0ea;
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.86);
}

.layout.dark .sidebar {
  border-color: #2f3d53;
  background: rgba(15, 23, 42, 0.7);
}

.sidebar-splitter {
  min-height: 0;
  border-radius: 10px;
  background: linear-gradient(to bottom, rgba(148, 163, 184, 0.45), rgba(148, 163, 184, 0.2));
  border: 1px solid rgba(148, 163, 184, 0.45);
  cursor: col-resize;
  transition: background 120ms ease, border-color 120ms ease;
}

.sidebar-splitter:hover {
  background: linear-gradient(to bottom, rgba(34, 197, 94, 0.35), rgba(16, 185, 129, 0.2));
  border-color: rgba(16, 185, 129, 0.55);
}

.layout.dark .sidebar-splitter {
  background: linear-gradient(to bottom, rgba(71, 85, 105, 0.6), rgba(51, 65, 85, 0.4));
  border-color: rgba(100, 116, 139, 0.45);
}

.layout.dark .sidebar-splitter:hover {
  background: linear-gradient(to bottom, rgba(16, 185, 129, 0.45), rgba(34, 197, 94, 0.25));
  border-color: rgba(16, 185, 129, 0.62);
}

.sidebar-section-title {
  padding: 8px 11px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #64748b;
  border-bottom: 1px solid #e4eaf2;
  background: rgba(248, 251, 255, 0.88);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.section-actions {
  display: inline-flex;
  gap: 6px;
}

.mini-action {
  border: 1px solid #ccd6e4;
  border-radius: 7px;
  background: #ffffff;
  color: #334155;
  padding: 2px 6px;
  font-size: 10px;
  cursor: pointer;
}

.mini-action:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.layout.dark .sidebar-section-title {
  color: #93a4bc;
  border-bottom-color: #334155;
  background: rgba(15, 23, 42, 0.8);
}

.layout.dark .mini-action {
  border-color: #42536d;
  background: #1b2537;
  color: #d3deec;
}

.recent-title {
  border-top: 1px solid #e4eaf2;
}

.layout.dark .recent-title {
  border-top-color: #334155;
}

.sidebar-list {
  overflow: auto;
  padding: 8px 7px;
  display: grid;
  gap: 6px;
  align-content: flex-start;
}

.sidebar-search-wrap {
  padding: 7px 7px 0;
}

.sidebar-search {
  width: 100%;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 6px 8px;
  background: #ffffff;
  color: #0f172a;
  font-size: 12px;
}

.layout.dark .sidebar-search {
  border-color: #475569;
  background: #111827;
  color: #e5e7eb;
}

.outline-row {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: center;
  gap: 6px;
  border-radius: 8px;
}

.outline-row.active {
  background: rgba(144, 163, 188, 0.18);
}

.layout.dark .outline-row.active {
  background: rgba(56, 189, 248, 0.16);
}

.outline-toggle {
  width: 16px;
  height: 20px;
  border: 0;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  padding: 0;
  line-height: 1;
  font-size: 11px;
}

.outline-toggle.placeholder {
  cursor: default;
  color: transparent;
}

.layout.dark .outline-toggle {
  color: #94a3b8;
}

.recent-list {
  max-height: 180px;
}

.recent-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 6px;
  align-items: center;
  border-radius: 8px;
}

.recent-row.dragging {
  opacity: 0.55;
  background: rgba(148, 163, 184, 0.16);
}

.recent-open {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-item {
  width: 100%;
  text-align: left;
}

.outline-title {
  border: 0;
  background: transparent;
  padding: 4px 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-item.active {
  background: #e8f0fb;
  border-color: #9fb1c9;
  font-weight: 600;
}

.layout.dark .sidebar-item.active {
  background: #2a3a54;
  border-color: #6a7f9f;
}

.mini-action.danger {
  color: #991b1b;
}

.layout.dark .mini-action.danger {
  color: #fca5a5;
}

.mini-action.accent {
  color: #a16207;
}

.layout.dark .mini-action.accent {
  color: #facc15;
}

.sidebar-empty {
  margin: 8px;
  font-size: 12px;
  color: #64748b;
}

.layout.dark .sidebar-empty {
  color: #94a3b8;
}

.doc-zone {
  height: 100%;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(320px, 1fr) 8px minmax(320px, 1fr);
  gap: 10px;
}

.doc-zone.mode-edit,
.doc-zone.mode-preview {
  grid-template-columns: 1fr;
}

.hidden {
  display: none;
}

.splitter {
  min-height: 0;
  border-radius: 10px;
  background: linear-gradient(to bottom, rgba(148, 163, 184, 0.45), rgba(148, 163, 184, 0.2));
  border: 1px solid rgba(148, 163, 184, 0.45);
  cursor: col-resize;
  transition: background 120ms ease, border-color 120ms ease;
}

.splitter:hover {
  background: linear-gradient(to bottom, rgba(56, 189, 248, 0.4), rgba(14, 165, 233, 0.25));
  border-color: rgba(14, 165, 233, 0.55);
}

.layout.dark .splitter {
  background: linear-gradient(to bottom, rgba(71, 85, 105, 0.6), rgba(51, 65, 85, 0.4));
  border-color: rgba(100, 116, 139, 0.45);
}

.layout.dark .splitter:hover {
  background: linear-gradient(to bottom, rgba(14, 165, 233, 0.45), rgba(56, 189, 248, 0.28));
  border-color: rgba(56, 189, 248, 0.62);
}

.editor-pane,
.preview-pane {
  height: 100%;
  min-height: 0;
  border: 1px solid #d9e0ea;
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.9);
}

.layout.dark .editor-pane,
.layout.dark .preview-pane {
  border-color: #2f3d53;
  background: rgba(15, 23, 42, 0.76);
}

.editor-host {
  height: 100%;
  min-height: 0;
}

.preview {
  height: 100%;
  overflow: auto;
  padding: 16px 18px;
  font-family: var(--nmd-editor-font), "PingFang SC", "Microsoft YaHei", sans-serif;
}

.statusbar {
  grid-row: 5;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  min-height: 34px;
  max-height: 52px;
  padding: 7px 11px;
  border: 1px solid #d9e0ea;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.84);
  font-size: 12px;
  line-height: 1.25;
  color: #475569;
  overflow: hidden;
  white-space: normal;
}

.layout.dark .statusbar {
  border-color: #2f3d53;
  background: rgba(15, 23, 42, 0.74);
  color: #cbd5e1;
}

.status-main {
  min-width: 0;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

.status-meta {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
  max-height: calc(2 * 1.25em + 8px);
  overflow: hidden;
  min-width: 0;
}

.status-meta span {
  white-space: nowrap;
}

.status-error {
  color: #b91c1c;
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  text-align: left;
}

.layout.dark .status-error {
  color: #fca5a5;
}

.autosave-error-panel {
  width: min(720px, calc(100vw - 30px));
}

.autosave-error-filters {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.autosave-filter-btn {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.autosave-filter-btn.active {
  background: #e9f0fb;
  border-color: #91a5c2;
}

.autosave-filter-btn.danger {
  border-color: #ef4444;
  color: #b91c1c;
}

.autosave-filter-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.autosave-error-search {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  padding: 4px 10px;
  font-size: 12px;
  min-width: 180px;
}

.autosave-error-grid {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 10px;
}

.autosave-error-list {
  border: 1px solid #d6dde8;
  border-radius: 10px;
  background: #f8fbff;
  overflow: auto;
  max-height: 40vh;
  padding: 6px;
  display: grid;
  gap: 6px;
  align-content: flex-start;
}

.autosave-error-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 6px;
  align-items: center;
}

.autosave-error-check {
  width: 14px;
  height: 14px;
  margin: 0 0 0 2px;
}

.autosave-error-item {
  border: 1px solid #ccd6e4;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  padding: 6px 8px;
  text-align: left;
  display: grid;
  gap: 2px;
  font-size: 11px;
  cursor: pointer;
  width: 100%;
}

.autosave-error-row.active .autosave-error-item {
  background: #e9f0fb;
  border-color: #91a5c2;
}

.autosave-error-detail {
  margin: 0;
  padding: 10px;
  border: 1px solid #d6dde8;
  border-radius: 10px;
  background: #f8fbff;
  color: #334155;
  max-height: 40vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
}

.layout.dark .autosave-error-detail {
  border-color: #334155;
  background: #0f172a;
  color: #d3deec;
}

.layout.dark .autosave-error-list {
  border-color: #334155;
  background: #0f172a;
}

.layout.dark .autosave-filter-btn {
  border-color: #42536d;
  background: #1b2537;
  color: #d3deec;
}

.layout.dark .autosave-filter-btn.active {
  background: #2b3d5a;
  border-color: #6e84a5;
}

.layout.dark .autosave-filter-btn.danger {
  border-color: #f87171;
  color: #fca5a5;
}

.layout.dark .autosave-error-search {
  border-color: #42536d;
  background: #1b2537;
  color: #d3deec;
}

.layout.dark .autosave-error-item {
  border-color: #42536d;
  background: #1b2537;
  color: #d3deec;
}

.layout.dark .autosave-error-row.active .autosave-error-item {
  background: #2b3d5a;
  border-color: #6e84a5;
}

.palette-mask {
  position: fixed;
  inset: 0;
  z-index: 200;
  background: rgba(15, 23, 42, 0.35);
  display: grid;
  place-items: center;
}

.palette {
  width: min(480px, calc(100vw - 30px));
  max-height: calc(100vh - 30px);
  border: 1px solid #d6dde8;
  border-radius: 12px;
  background: #ffffff;
  padding: 14px;
  display: grid;
  gap: 8px;
  overflow: auto;
}

.layout.dark .palette {
  border-color: #334155;
  background: #111827;
}

.palette h2 {
  margin: 0 0 4px;
  font-size: 16px;
  color: inherit;
}

.palette-search {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 8px 10px;
  background: #ffffff;
  color: #0f172a;
}

.layout.dark .palette-search {
  border-color: #475569;
  background: #1f2937;
  color: #e5e7eb;
}

.shortcut {
  margin-left: 6px;
  font-size: 11px;
  color: #64748b;
}

.layout.dark .shortcut {
  color: #94a3b8;
}

.palette-empty {
  margin: 4px 2px;
  font-size: 12px;
  color: #64748b;
}

.layout.dark .palette-empty {
  color: #94a3b8;
}

.palette-list {
  max-height: 58vh;
  overflow: auto;
  display: grid;
  gap: 8px;
}

.help-groups {
  max-height: 58vh;
  overflow: auto;
  display: grid;
  gap: 8px;
}

.help-group {
  border: 1px solid #d6dde8;
  border-radius: 10px;
  padding: 8px;
  display: grid;
  gap: 8px;
  background: #fbfdff;
}

.help-group-toggle {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.help-list {
  display: grid;
  gap: 8px;
}

.help-list button.active {
  background: #e9f0fb;
  border-color: #91a5c2;
}

.usage-panel {
  width: min(860px, calc(100vw - 30px));
}

.settings-panel {
  width: min(920px, calc(100vw - 30px));
}

.settings-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.settings-section {
  border: 1px solid #d6dde8;
  border-radius: 10px;
  padding: 10px;
  display: grid;
  gap: 8px;
  align-content: flex-start;
  background: #fbfdff;
}

.settings-section h3 {
  margin: 0;
  font-size: 13px;
}

.settings-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.settings-check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.settings-row select {
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  color: inherit;
  padding: 5px 8px;
  font-size: 12px;
}

.settings-shortcut-input {
  width: 170px;
  border: 1px solid #cfd8e5;
  border-radius: 8px;
  padding: 6px 8px;
  background: #ffffff;
  color: inherit;
  cursor: pointer;
}

.settings-shortcut-input:disabled {
  opacity: 0.5;
}

.settings-shortcut-input.invalid {
  border-color: #dc2626;
  box-shadow: 0 0 0 1px rgba(220, 38, 38, 0.15);
}

.settings-row-action {
  padding: 5px 8px;
  font-size: 12px;
}

.settings-warning {
  margin: 0;
  font-size: 12px;
  color: #b91c1c;
}

.usage-body {
  border: 1px solid #d6dde8;
  border-radius: 10px;
  padding: 12px;
  max-height: 66vh;
  overflow: auto;
  background: #fbfdff;
}

.layout.dark .usage-body {
  border-color: #334155;
  background: #0f172a;
}

.layout.dark .settings-section {
  border-color: #334155;
  background: #0f172a;
}

.layout.dark .help-group {
  border-color: #334155;
  background: #0f172a;
}

.layout.dark .help-list button.active {
  background: #2b3d5a;
  border-color: #6e84a5;
}

.layout.dark .settings-shortcut-input {
  border-color: #334155;
  background: #111827;
}

.layout.dark .settings-row select {
  border-color: #334155;
  background: #111827;
}

.layout.dark .settings-shortcut-input.invalid {
  border-color: #ef4444;
  box-shadow: 0 0 0 1px rgba(248, 113, 113, 0.25);
}

.layout.dark .settings-warning {
  color: #fca5a5;
}

.preview :deep(h1),
.preview :deep(h2),
.preview :deep(h3) {
  color: inherit;
}

.preview :deep(blockquote) {
  margin: 14px 0;
  padding: 8px 14px;
  border-left: 3px solid #9fb3cc;
  background: #f4f7fb;
  color: #4b5563;
  border-radius: 0 8px 8px 0;
}

.preview :deep(blockquote p) {
  margin: 0.4em 0;
}

.preview :deep(pre) {
  border-radius: 8px;
  padding: 12px;
  background: #f3f5f9;
}

.layout.dark .preview :deep(pre) {
  background: #0b1220;
}

.layout.dark .preview :deep(blockquote) {
  border-left-color: #5e7594;
  background: #182337;
  color: #c3cfde;
}

.preview :deep(code) {
  font-family: "JetBrains Mono", "SF Mono", "Menlo", monospace;
}

.preview :deep(img) {
  max-width: 100%;
  height: auto;
  display: block;
  border-radius: 6px;
  margin: 8px 0;
}

.preview :deep(.task-list-item) {
  list-style: none;
}

.preview :deep(.task-list-item-checkbox) {
  margin-right: 8px;
  transform: translateY(1px);
}

@media (max-width: 1200px) {
  .workspace {
    grid-template-columns: 1fr;
  }

  .sidebar-splitter {
    display: none;
  }

  .doc-zone {
    grid-template-columns: 1fr;
  }

  .splitter {
    display: none;
  }

  .sidebar {
    grid-template-rows: auto minmax(120px, 1fr) auto minmax(100px, 1fr);
  }
}
</style>
