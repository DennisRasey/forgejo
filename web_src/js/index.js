// bootstrap module must be the first one to be imported, it handles webpack lazy-loading and global errors
import './bootstrap.js';

import {initRepoActivityTopAuthorsChart} from './features/repo-activity-top-authors.ts';
import {initDashboardRepoList} from './features/dashboard-repo-list.ts';

import {initGlobalCopyToClipboardListener} from './features/clipboard.js';
import {initContextPopups} from './features/contextpopup.js';
import {initRepoGraphGit} from './features/repo-graph.js';
import {initHeatmap} from './features/heatmap.js';
import {initImageDiff} from './features/imagediff.js';
import {initRepoMigration} from './features/repo-migration.js';
import {initRepoProject} from './features/repo-projects.js';
import {initTableSort} from './features/tablesort.js';
import {initAutoFocusEnd} from './features/autofocus-end.js';
import {initAdminUserListSearchForm} from './features/admin/users.js';
import {initAdminConfigs} from './features/admin/config.js';
import {initMarkupAnchors} from './markup/anchors.js';
import {initNotificationCount, initNotificationsTable} from './features/notification.js';
import {initRepoIssueContentHistory} from './features/repo-issue-content.js';
import {initStopwatch} from './features/stopwatch.js';
import {initFindFileInRepo} from './features/repo-findfile.js';
import {initCommentContent, initMarkupContent} from './markup/content.js';

import {initUserAuthOauth2, initUserAuth} from './features/user-auth.js';
import {
  initRepoIssueDue,
  initRepoIssueReferenceRepositorySearch,
  initRepoIssueTimeTracking,
  initRepoIssueWipTitle,
  initRepoPullRequestAllowMaintainerEdit,
  initRepoPullRequestReview,
  initArchivedLabelHandler,
} from './features/repo-issue.js';
import {initRepoIssueSidebarList} from './features/repo-issue-sidebar-list.ts';
import {initRepoEllipsisButton, initCommitStatuses, initCommitNotes} from './features/repo-commit.js';
import {
  initFootLanguageMenu,
  initGlobalButtonClickOnEnter,
  initGlobalButtons,
  initGlobalCommon,
  initDisabledInputs,
  initGlobalDropzone,
  initGlobalEnterQuickSubmit,
  initGlobalFormDirtyLeaveConfirm,
  initGlobalLinkActions,
  initHeadNavbarContentToggle,
} from './features/common-global.js';
import {initRepoTopicBar} from './features/repo-home.js';
import {initAdminEmails} from './features/admin/emails.js';
import {initAdminCommon} from './features/admin/common.js';
import {initRepoTemplateSearch} from './features/repo-template.js';
import {initRepoCodeView} from './features/repo-code.ts';
import {initSshKeyFormParser} from './features/sshkey-helper.js';
import {initRepoArchiveLinks} from './features/repo-common.js';
import {initRepoMigrationStatusChecker} from './features/repo-migrate.js';
import {
  initRepoSettingGitHook,
  initRepoSettingsCollaboration,
  initRepoSettingSearchTeamBox,
} from './features/repo-settings.js';
import {initRepoDiffView} from './features/repo-diff.js';
import {initRepoSuggestionApply} from './features/repo-suggestion.js';
import {initOrgTeamSearchRepoBox} from './features/org-team.js';
import {initUserAuthWebAuthn, initUserAuthWebAuthnRegister} from './features/user-auth-webauthn.js';
import {initRepoRelease, initRepoReleaseNew} from './features/repo-release.js';
import {initRepoEditor} from './features/repo-editor.js';
import {initAddOrgMemberButton} from './features/add-org-member.ts';
import {initCompSearchUserBox} from './features/comp/SearchUserBox.js';
import {initInstall} from './features/install.js';
import {initCompWebHookEditor} from './features/comp/WebHookEditor.js';
import {initRepoBranchButton} from './features/repo-branch.ts';
import {initCommonOrganization} from './features/common-organization.js';
import {initRepoWikiForm} from './features/repo-wiki.js';
import {initRepoCommentForm, initRepository} from './features/repo-legacy.js';
import {initCopyContent} from './features/copycontent.js';
import {initCaptcha} from './features/captcha.js';
import {initRepositoryActionView} from './features/repo-action-view.ts';
import {initGlobalTooltips} from './modules/tippy.js';
import {initDropdowns} from './modules/dropdown.ts';
import {initGiteaFomantic} from './modules/fomantic.js';
import {onDomReady} from './utils/dom.js';
import {initRepoIssueList} from './features/repo-issue-list.js';
import {initCommonIssueListQuickGoto} from './features/common-issue-list.js';
import {initRepoContributors} from './features/contributors.js';
import {initRepoCodeFrequency} from './features/code-frequency.js';
import {initRepoRecentCommits} from './features/recent-commits.js';
import {initRepoDiffCommitBranchesAndTags} from './features/repo-diff-commit.js';
import {initDirAuto} from './modules/dirauto.js';
import {initRepositorySearch} from './features/repo-search.js';
import {initColorPickers} from './features/colorpicker.js';
import {initRepoMilestoneEditor} from './features/repo-milestone.js';
import {initModalClose} from './modules/modal.ts';
import {initAuthorizedIntegrationClaimRuleEditor} from './features/authorized-integration.js';

// Init Gitea's Fomantic settings
initGiteaFomantic();
initDirAuto();

onDomReady(() => {
  initGlobalCommon();

  initDisabledInputs(document);
  initGlobalTooltips();
  initGlobalButtonClickOnEnter();
  initGlobalButtons();
  initGlobalCopyToClipboardListener();
  initGlobalDropzone();
  initGlobalEnterQuickSubmit();
  initGlobalFormDirtyLeaveConfirm();
  initGlobalLinkActions();
  initDropdowns();

  initCommonOrganization();
  initCommonIssueListQuickGoto();

  initCompSearchUserBox();
  initCompWebHookEditor();

  initInstall();

  initHeadNavbarContentToggle();
  initFootLanguageMenu();

  initCommentContent();
  initContextPopups();
  initHeatmap();
  initImageDiff();
  initMarkupAnchors();
  initMarkupContent();
  initSshKeyFormParser();
  initStopwatch();
  initTableSort();
  initAutoFocusEnd();
  initFindFileInRepo();
  initCopyContent();

  initAdminCommon();
  initAdminEmails();
  initAdminUserListSearchForm();
  initAdminConfigs();

  initDashboardRepoList();

  initNotificationCount();
  initNotificationsTable();

  initOrgTeamSearchRepoBox();

  initRepoActivityTopAuthorsChart();
  initRepoArchiveLinks();
  initRepoBranchButton();
  initRepoCodeView();
  initRepoCommentForm();
  initRepoEllipsisButton();
  initRepoDiffCommitBranchesAndTags();
  initRepoEditor();
  initAddOrgMemberButton();
  initRepoGraphGit();
  initRepoIssueContentHistory();
  initRepoIssueDue();
  initRepoIssueList();
  initRepoIssueSidebarList();
  initArchivedLabelHandler();
  initRepoIssueReferenceRepositorySearch();
  initRepoIssueTimeTracking();
  initRepoIssueWipTitle();
  initRepoMigration();
  initRepoMigrationStatusChecker();
  initRepoProject();
  initRepoPullRequestAllowMaintainerEdit();
  initRepoPullRequestReview();
  initRepoRelease();
  initRepoReleaseNew();
  initRepoSettingGitHook();
  initRepoSettingSearchTeamBox();
  initRepoSettingsCollaboration();
  initRepoTemplateSearch();
  initRepoTopicBar();
  initRepoWikiForm();
  initRepository();
  initRepositoryActionView();
  initRepositorySearch();
  initRepoContributors();
  initRepoCodeFrequency();
  initRepoRecentCommits();
  initRepoMilestoneEditor();

  initCommitStatuses();
  initCommitNotes();
  initCaptcha();

  initUserAuthOauth2();
  initUserAuthWebAuthn();
  initUserAuthWebAuthnRegister();
  initUserAuth();
  initRepoDiffView();
  initRepoSuggestionApply();
  initColorPickers();
  initModalClose();
  initAuthorizedIntegrationClaimRuleEditor();

  // Deactivate CSS-only noJS usability supplements
  document.body.classList.remove('no-js');
});
