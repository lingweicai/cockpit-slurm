/*
 * SPDX-License-Identifier: LGPL-2.1-or-later
 *
 * Copyright (C) 2017 Red Hat, Inc.
 */

import React from 'react';
import { createRoot } from 'react-dom/client';

import "cockpit-dark-theme";

import { Application } from './app.jsx';

// @ts-ignore: Cannot find module or type declarations for side-effect import
import "patternfly/patternfly-6-cockpit.scss";
// @ts-ignore: Cannot find module or type declarations for side-effect import
import './app.scss';

document.addEventListener("DOMContentLoaded", () => {
    createRoot(document.getElementById("app")!).render(<Application />);
});
