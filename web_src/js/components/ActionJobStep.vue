<!--
Copyright 2025 The Forgejo Authors. All rights reserved.
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script>
import {SvgIcon} from '../svg.js';
import ActionRunStatus from './ActionRunStatus.vue';
import {toggleElem} from '../utils/dom.js';
import {formatDatetime} from '../utils/time.js';
import {renderAnsiWithLinks} from '../render/ansi.js';

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
      const div = document.createElement('div');
      div.classList.add('job-log-line');
      div.setAttribute('id', `jobstep-${this.stepId}-${lineNo}`);
      div._jobLogTime = line.timestamp;

      const lineNumber = document.createElement('a');
      lineNumber.classList.add('line-num', 'muted');
      lineNumber.textContent = lineNo;
      lineNumber.setAttribute('href', `#jobstep-${this.stepId}-${lineNo}`);
      div.append(lineNumber);

      // for "Show timestamps"
      const logTimeStamp = document.createElement('span');
      logTimeStamp.className = 'log-time-stamp';
      const date = new Date(parseFloat(line.timestamp * 1000));
      const timeStamp = formatDatetime(date);
      logTimeStamp.textContent = timeStamp;
      toggleElem(logTimeStamp, this.timeVisibleTimestamp);
      // for "Show seconds"
      const logTimeSeconds = document.createElement('span');
      logTimeSeconds.className = 'log-time-seconds';
      const seconds = Math.floor(parseFloat(line.timestamp) - parseFloat(startTime));
      logTimeSeconds.textContent = `${seconds}s`;
      toggleElem(logTimeSeconds, this.timeVisibleSeconds);

      let logMessage = document.createElement('span');
      logMessage.innerHTML = renderAnsiWithLinks(line.message);
      // If the input to renderAnsi is not empty and the output is empty we can
      // assume the input was only ANSI escape codes that have been removed. In
      // that case we should not display this message
      if (line.message !== '' && logMessage.innerHTML === '') {
        this.lineNumberOffset++;
        return [];
      }
      if (group.isHeader) {
        const details = document.createElement('details');
        details.addEventListener('toggle', this.toggleGroupLogs);
        const summary = document.createElement('summary');
        summary.append(logMessage);
        details.append(summary);
        logMessage = details;
      }
      logMessage.className = 'log-msg';
      logMessage.style.paddingLeft = `${group.depth}em`;

      div.append(logTimeStamp);
      div.append(logMessage);
      div.append(logTimeSeconds);

      return div;
    },

    appendLogs(logLines, startTime) {
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
