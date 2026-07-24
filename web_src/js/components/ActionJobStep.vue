<!--
Copyright 2025 The Forgejo Authors. All rights reserved.
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script>
import {SvgIcon} from '../svg.js';
import ActionRunStatus from './ActionRunStatus.vue';
import {formatDatetime} from '../utils/time.js';
import {renderAnsiWithLinks} from '../render/ansi.js';
import {htmlEscape} from 'escape-goat';

// show/hide the step logs for a group
window.actionJobStepToggleGroupLogs = function(event) {
  const line = event.target.parentElement;
  const list = line.nextSibling;
  list.classList.toggle('hidden', event.newState !== 'open');
};

export default {
  name: 'ActionJobStep',
  components: {
    SvgIcon,
    ActionRunStatus,
  },
  props: {
    stepId: {
      type: Number,
      required: true,
    },
    status: {
      type: String,
      required: true,
    },
    runStatus: {
      type: String,
      required: true,
    },
    expanded: {
      type: Boolean,
      required: true,
    },
    isExpandable: {
      type: Function,
      required: true,
    },
    isDone: {
      type: Function,
      required: true,
    },
    cursor: {
      type: Number,
      required: false,
      default: null,
    },
    summary: {
      type: String,
      required: true,
    },
    duration: {
      type: String,
      required: true,
    },
    timeVisibleTimestamp: {
      type: Boolean,
      required: true,
    },
    timeVisibleSeconds: {
      type: Boolean,
      required: true,
    },
  },
  emits: ['toggle'],

  data() {
    return {
      lineNumberOffset: 0,
    };
  },

  methods: {
    createLogLine(line, startTime, group) {
      const lineNo = line.index - this.lineNumberOffset;

      // Text chunk HTML construction is used here because it is faster than creating multiple DOM nodes, which can be
      // relevant when viewing large collections of logs.  Special care must be taken to ensure that none of the data
      // allows unescaped used-generated content, or else XSS-style vulnerabilities may exist.
      const chunks = [];
      chunks.push(
        `<div class="job-log-line" id="jobstep-${this.stepId}-${lineNo}">`,
        `<a class="line-num muted" href="#jobstep-${this.stepId}-${lineNo}">${lineNo}</a>`,
      );

      // for "Show timestamps"
      const date = new Date(parseFloat(line.timestamp * 1000));
      const timeStamp = htmlEscape(formatDatetime(date));
      const timeStampHidden = this.timeVisibleTimestamp ? '' : 'tw-hidden';
      chunks.push(`<span class="log-time-stamp ${timeStampHidden}">${timeStamp}</span>`);

      const renderedMessage = renderAnsiWithLinks(line.message);
      // If the input to renderAnsi is not empty and the output is empty we can
      // assume the input was only ANSI escape codes that have been removed. In
      // that case we should not display this message
      if (line.message !== '' && renderedMessage === '') {
        this.lineNumberOffset++;
        return [];
      }

      if (group.isHeader) {
        chunks.push(`<details class="log-msg" style="padding-left: ${group.depth}em" ontoggle="actionJobStepToggleGroupLogs(event)"><summary><span>${renderedMessage}</span></summary></details>`);
      } else {
        chunks.push(`<span class="log-msg" style="padding-left: ${group.depth}em">${renderedMessage}</span>`);
      }

      // for "Show seconds"
      const secondsHidden = this.timeVisibleSeconds ? '' : 'tw-hidden';
      const seconds = Math.max(Math.floor(parseFloat(line.timestamp) - parseFloat(startTime), 0));
      chunks.push(
        `<span class="log-time-seconds ${secondsHidden}">${seconds}s</span>`,
        '</div>',
      );

      const tmpl = document.createElement('template');
      tmpl.innerHTML = chunks.join('');
      return tmpl.content.firstElementChild;
    },

    async appendLogs(logLines, startTime) {
      this.lineNumberOffset = 0;

      const groupStack = [];
      const container = this.$refs.logsContainer;
      for (const line of logLines) {
        const el = groupStack.length > 0 ? groupStack[groupStack.length - 1] : container;
        const group = {
          depth: groupStack.length,
          isHeader: false,
        };
        if (line.message.startsWith('##[group]')) {
          group.isHeader = true;

          const logLine = this.createLogLine(
            {
              ...line,
              message: line.message.substring(9),
            },
            startTime, group,
          );
          logLine.setAttribute('data-group', group.index);
          el.append(logLine);

          const list = document.createElement('div');
          list.classList.add('job-log-list', 'hidden');
          list.setAttribute('data-group', group.index);
          groupStack.push(list);
          el.append(list);
        } else if (line.message.startsWith('##[endgroup]')) {
          groupStack.pop();
        } else {
          el.append(this.createLogLine(line, startTime, group));
        }

        // When a user opens up a completed action step with many (100k+) log entries, we can end up invoking
        // `appendLogs` with big chunks of data.  When a long JS tasks runs, it causes the browser UI to freeze up.  In
        // order to provide a responsive user experience, invoke `scheduler.yield()` occasionally to allow the browser
        // to suspend this task, layout and display the contents that have been updated, and then return to the task to
        // continue appending more log lines.  Every 1000 lines is an frequency derived from experimental testing when
        // viewing 100,000 log lines -- the more we yield the longer the overall process takes, and the less we yield
        // the more the UI appears frozen.
        //
        // `scheduler.yield` is not supported in Safari, so its availability is checked before executing.
        //
        // The downside of yielding is that the logs appear in the browser while we're still appending them -- if you
        // immediately do a "Find in page" search for something, you might not find results that are present and just
        // haven't been rendered yet.
        if ((line.index % 1000) === 0 && typeof scheduler !== 'undefined' && typeof scheduler.yield === 'function') {
          await scheduler.yield();
        }
      }
    },

    // show/hide the step logs for a group
    toggleGroupLogs(event) {
      const line = event.target.parentElement;
      const list = line.nextSibling;
      list.classList.toggle('hidden', event.newState !== 'open');
    },

    scrollIntoView(lineID) {
      const logLine = this.$refs.logsContainer.querySelector(lineID);
      if (!logLine) {
        return;
      }
      logLine.querySelector('.line-num').scrollIntoView();
    },
  },
};
</script>
<template>
  <div
    class="job-step-summary"
    tabindex="0"
    @click.stop="isExpandable(status) && $emit('toggle')"
    @keyup.enter.stop="isExpandable(status) && $emit('toggle')"
    @keyup.space.stop="isExpandable(status) && $emit('toggle')"
    :class="[expanded ? 'selected' : '', isExpandable(status) && 'step-expandable']"
  >
    <!-- If the job is done and the job step log is loaded for the first time, show the loading icon
      currentJobStepsStates[i].cursor === null means the log is loaded for the first time
    -->
    <SvgIcon
      v-if="isDone(runStatus) && expanded && cursor === null"
      name="octicon-sync"
      class="tw-mr-2 job-status-rotate"
    />
    <SvgIcon
      v-else
      :name="expanded ? 'octicon-chevron-down': 'octicon-chevron-right'"
      :class="['tw-mr-2', !isExpandable(status) && 'tw-invisible']"
    />
    <ActionRunStatus :status="status" class="tw-mr-2"/>

    <span class="step-summary-msg gt-ellipsis">{{ summary }}</span>
    <span class="step-summary-duration">{{ duration }}</span>
  </div>

  <!-- the log elements could be a lot, do not use v-if to destroy/reconstruct the DOM,
  use native DOM elements for "log line" to improve performance, Vue is not suitable for managing so many reactive elements. -->
  <div class="job-step-logs" ref="logsContainer" v-show="expanded"/>
</template>
<style scoped>

.job-step-summary {
  padding: 5px 10px;
  display: flex;
  align-items: center;
  border-radius: var(--border-radius);
}

.job-step-summary.step-expandable {
  cursor: pointer;
}

.job-step-summary.step-expandable:hover {
  color: var(--color-console-fg);
  background: var(--color-console-hover-bg);
}

.job-step-summary .step-summary-msg {
  flex: 1;
}

.job-step-summary .step-summary-duration {
  margin-inline-start: 16px;
}

.job-step-summary.selected {
  color: var(--color-console-fg);
  background-color: var(--color-console-active-bg);
  position: sticky;
  top: 60px;
}

.job-step-logs {
  font-family: var(--fonts-monospace);
  margin: 8px 0;
  font-size: 12px;
}

</style>
<style>
/* some elements are not managed by vue, so we need to use global style */

.job-step-logs .job-log-line {
  display: flex;
}

.job-step-logs .job-log-line .log-msg {
  flex: 1;
  word-break: break-all;
  white-space: break-spaces;
  margin-inline-start: 10px;
  overflow-wrap: anywhere;
  color: var(--color-console-fg);
}

.job-log-line:hover,
.job-log-line:target {
  background-color: var(--color-console-hover-bg);
}

.job-log-line:target {
  scroll-margin-top: 95px;
}

/* class names 'log-time-seconds' and 'log-time-stamp' are used in the method toggleTimeDisplay */
.job-log-line .line-num, .log-time-seconds {
  width: 48px;
  color: var(--color-text-light-3);
  text-align: end;
  user-select: none;
}

.job-log-line:target > .line-num {
  color: var(--color-primary);
  text-decoration: underline;
}

.log-time-seconds {
  padding-inline-end: 2px;
}

.job-log-line .log-time,
.log-time-stamp {
  color: var(--color-text-light-3);
  margin-inline-start: 10px;
  white-space: nowrap;
}

</style>
