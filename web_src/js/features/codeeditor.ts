import type {EditorView} from '@codemirror/view';
import {basename, extname} from '../utils.js';
import {hideElem, onInputDebounce, showElem} from '../utils/dom.js';
import {createCodemirror, type CodemirrorEditor, type EditorOptions} from './codemirror.ts';
import type {LanguageSupport} from '@codemirror/language';

interface EditorConfig {
  indent_style: string;
  indent_size: string;
}

function getEditorconfig(input: HTMLInputElement): null | EditorConfig {
  try {
    return JSON.parse(input.getAttribute('data-editorconfig'));
  } catch {
    return null;
  }
}

async function updateEditor(editor: CodemirrorEditor, filename: string, lineWrapExts: string[]) {
  const fileOption = getFileBasedOptions(filename, lineWrapExts);
  editor.view.dispatch({
    effects: editor.compartments.wordWrap.reconfigure(fileOption.wordWrap ? editor.codemirrorView.EditorView.lineWrapping : []),
  });

  const currentLanguage = editor.compartments.language.get(editor.view.state) as Array<unknown> | LanguageSupport;
  const newLanguage = editor.codemirrorLanguage.LanguageDescription.matchFilename(editor.languages, filename);
  if (!currentLanguage || (currentLanguage as Array<unknown>).length === 0 || !newLanguage || (currentLanguage as LanguageSupport).language.name.toLowerCase() !== newLanguage.name.toLowerCase()) {
    editor.view.dispatch({
      effects: editor.compartments.language.reconfigure(newLanguage ? await newLanguage.load() : []),
    });
  }
}

function getFileBasedOptions(filename: string, lineWrapExts: string[]): Pick<EditorOptions, 'wordWrap'> {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)),
  };
}

function togglePreviewDisplay(previewable: boolean) {
  const previewTab = document.querySelector('.item[data-tab="preview"]') as HTMLAnchorElement;
  if (!previewTab) return;

  if (previewable) {
    const newUrl = (previewTab.getAttribute('data-url') || '').replace(/(.*)\/.*/, `$1/markup`);
    previewTab.setAttribute('data-url', newUrl);
    previewTab.style.display = '';
  } else {
    previewTab.style.display = 'none';
    // If the "preview" tab was active, user changes the filename to a non-previewable one,
    // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
    if (previewTab.classList.contains('active')) {
      const writeTab = document.querySelector('.item[data-tab="write"]') as HTMLAnchorElement;
      writeTab.click();
    }
  }
}

export async function createCodeEditor(textarea: HTMLTextAreaElement, filenameInput: HTMLInputElement): Promise<EditorView> {
  const filename = basename(filenameInput.value);
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');
  const previewable = previewableExts.has(extname(filename));
  const editorConfig = getEditorconfig(filenameInput);

  togglePreviewDisplay(previewable);

  const editor = await createCodemirror(textarea, filename, {
    ...getFileBasedOptions(filenameInput.value, lineWrapExts),
    ...getEditorConfigOptions(editorConfig),
  });

  filenameInput.addEventListener('input', onInputDebounce(async () => {
    const filename = filenameInput.value;
    const previewable = previewableExts.has(extname(filename));
    togglePreviewDisplay(previewable);
    await updateEditor(editor, filename, lineWrapExts);
  }));

  const searchButton = document.querySelector('#editor-find');
  searchButton.addEventListener('click', () => {
    const search = editor.codemirrorSearch;
    const view = editor.view;

    if (search.searchPanelOpen(view.state)) {
      search.closeSearchPanel(view);
    } else {
      search.openSearchPanel(view);
    }
  });

  const writeTab = document.querySelector('#editor-bar .switch .item[data-tab="write"]');
  document.querySelector('#editor-bar .switch').addEventListener('click', () => {
    if (writeTab.classList.contains('active')) {
      showElem(searchButton);
    } else {
      hideElem(searchButton);
    }
  });

  return editor.view;
}

function getEditorConfigOptions(ec: null | EditorConfig): Pick<EditorOptions, 'indentSize' | 'tabSize' | 'indentStyle'> {
  if (ec === null) {
    return {indentStyle: 'space'};
  }

  const opts: ReturnType<typeof getEditorConfigOptions> = {
    indentStyle: ec.indent_style,
  };
  if ('indent_size' in ec) opts.indentSize = Number(ec.indent_size);
  if ('tab_width' in ec) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
  return opts;
}
