interface Window {
  __webpack_public_path__: string;
  config?: {
    // These are set in template/base/head_script.tmpl
    readonly appUrl: string;
    readonly appSubUrl: string;
    readonly assetUrlPrefix: string;
    readonly pageData: PageData;
    readonly i18n: LocaleStrings;
    readonly customEmojis: ReadonlySet<string>;
    readonly mentionValues: readonly MentionValue[];
  }
}

interface LocaleStrings {
  readonly copy_success: string;
  readonly copy_error: string;
  readonly error_occurred: string;
  readonly network_error: string;
  readonly remove_label_str: string;
  readonly modal_confirm: string;
  readonly modal_cancel: string;
  readonly more_items: string;
  readonly incorrect_root_url: string;
}

interface PageData {
  readonly PLURALSTRINGS_LANG: Readonly<Record<string, readonly string[]>>;
  readonly PLURAL_RULE_LANG: number;
  readonly DATETIMESTRINGS: Readonly<Record<string, string>>;

  [key: string]: unknown; // TODO: remove this after we've enumerated all pageData entries
}

interface MentionValue {
  readonly key: string;
  readonly value: string;
  readonly name: string;
  readonly fullname: string;
  readonly avatar: string;
}

type CodeMirrorLanguage = typeof import('@codemirror/language');
type CodeMirrorSearch = typeof import('@codemirror/search');
type CodeMirrorState = typeof import('@codemirror/state');
type CodeMirrorView = typeof import('@codemirror/view');

declare module '*.vue' {
  import Vue from 'vue';
  export default Vue;
}

declare module '*.svg' {
  // webpack imports SVG files source-wise as strings via `type: asset/source`
  const svg: string;
  export default svg;
}

// until the plugin defines its own types, or we finish removing jquery:
declare module 'eslint-plugin-no-jquery' {
  import type {Plugin} from '@eslint/core';
  const plugin: Plugin;
  export = plugin;
}

// until https://github.com/freaktechnik/eslint-plugin-array-func/issues/492 is resolved:
declare module 'eslint-plugin-array-func' {
  // no-duplicate-imports: false-positive, moving this import to root makes this file a "module" and breaks the global-ness of the other declarations.
  // eslint-disable-next-line no-duplicate-imports
  import type {Plugin} from '@eslint/core';
  const plugin: Plugin;
  export default plugin;
}

// until https://github.com/dustinspecker/eslint-plugin-no-use-extend-native/issues/156 is resolved:
declare module 'eslint-plugin-no-use-extend-native' {
  import type {Plugin} from '@eslint/core'; // eslint-disable-line no-duplicate-imports
  const plugin: Plugin;
  export default plugin;
}
