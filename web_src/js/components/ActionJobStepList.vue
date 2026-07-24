<!--
Copyright 2025 The Forgejo Authors. All rights reserved.
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script>
import ActionJobStep from './ActionJobStep.vue';
import {toggleElem} from '../utils/dom.js';

export default {
  name: 'ActionJobStepList',
  components: {
    // SvgIcon,
    ActionJobStep,
  },
  props: {
    steps: {
      // Array of { status: string, summary: string, duration: string }
      type: Array,
      required: true,
    },
    stepStates: {
      // Array of { cursor: Number | null, expanded: boolean }.  Array length must match `steps`.
      type: Array,
      required: true,
    },
    runStatus: {
      type: String,
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
    timeVisibleTimestamp: {
      type: Boolean,
      required: true,
    },
    timeVisibleSeconds: {
      type: Boolean,
      required: true,
    },
  },

  emits: ['toggleStepLogs'],

  watch: {
    timeVisibleTimestamp(_oldVisible, _newVisible) {
      for (const el of this.$refs.steps.querySelectorAll(`.log-time-stamp`)) {
        toggleElem(el, this.timeVisibleTimestamp);
      }
    },
    timeVisibleSeconds(_oldVisible, _newVisible) {
      for (const el of this.$refs.steps.querySelectorAll(`.log-time-seconds`)) {
        toggleElem(el, this.timeVisibleSeconds);
      }
    },
  },

  methods: {
    async appendLogs(stepIndex, logLines, startTime) {
      await this.$refs.jobSteps[stepIndex].appendLogs(logLines, startTime);
    },
    scrollIntoView(stepIndex, lineID) {
      this.$refs.jobSteps[stepIndex].scrollIntoView(lineID);
    },
  },
};
</script>
<template>
  <div class="job-step-container" ref="steps" v-if="steps.length">
    <div class="job-step-section" v-for="(jobStep, i) in steps" :key="i">
      <ActionJobStep
        ref="jobSteps"
        :run-status="runStatus"
        :is-expandable="isExpandable"
        :is-done="isDone"
        :step-id="i"
        :status="jobStep.status"
        :summary="jobStep.summary"
        :duration="jobStep.duration"
        :expanded="stepStates[i].expanded"
        :cursor="stepStates[i].cursor"
        :time-visible-timestamp="timeVisibleTimestamp"
        :time-visible-seconds="timeVisibleSeconds"
        @toggle="() => $emit('toggleStepLogs', i)"
      />
    </div>
  </div>
</template>
<style scoped>

.job-step-container {
  max-height: 100%;
  border-radius: 0 0 var(--border-radius) var(--border-radius);
  border-top: 1px solid var(--color-console-border);
  z-index: 0;
}

.job-step-section {
  margin: 10px;
}

</style>
