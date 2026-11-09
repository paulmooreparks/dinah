// Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
import { createServer } from './json-server.mjs';
const server = createServer(message => self.postMessage(JSON.stringify(message)));
self.onmessage = event => { server.receive(event.data).catch(() => {}); };
