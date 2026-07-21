/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React from 'react';
import { createRoot } from 'react-dom/client';

import { Application } from './app';

import 'patternfly/patternfly-6-cockpit.scss';
import './app.scss';

const mountApplication = () => {
    const appRoot = document.getElementById('app');
    if (!appRoot) {
        console.error('cockpit-slurm mount failure: #app element not found');
        return;
    }

    createRoot(appRoot).render(<Application />);
};

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', mountApplication);
} else {
    mountApplication();
}