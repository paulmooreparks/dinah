// Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
const text = description => ({ type: 'string', description });
const choice = (values, description) => ({ type: 'string', enum: values, description });
const integer = description => ({ type: 'integer', minimum: -Number.MAX_SAFE_INTEGER, maximum: Number.MAX_SAFE_INTEGER, description });
const size = description => ({ ...integer(description), minimum: 0 });
const color = { type: 'string', pattern: '^(#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|[1-6])$', description: 'A hex color (#rgb or #rrggbb), or preset 1 through 6.' };
const side = choice(['top', 'right', 'bottom', 'left'], 'Attachment side. Omit for automatic attachment.');
const line = choice(['solid', 'dashed', 'dotted'], 'Outline or connector line style.');
const lineWidth = { type: 'integer', minimum: 1, maximum: 8, description: 'Line width in pixels.' };
const head = choice(['none', 'arrow', 'triangle', 'diamond', 'circle', 'box', 'cross', 'cf-one', 'cf-one-required', 'cf-many', 'cf-many-required'], 'Connector endpoint symbol.');
const auto = { type:'object',required:['node','side'],description:'Retains automatic attachment while the standard node and side match this saved projection.',properties:{node:{...text('Projected endpoint node ID.'),minLength:1},side} };
export const shapes = ['rectangle', 'rounded', 'oval', 'diamond', 'parallelogram', 'document', 'cylinder', 'queue', 'person', 'cloud', 'hexagon', 'package', 'text'];
export const canvasSchema = {
    $id: 'yavc-schema:json-canvas',
    type: 'object',
    description: 'JSON Canvas 1.0 with the optional yavc drawing extension. Unknown properties are preserved.',
    properties: {
        nodes: { type: 'array', description: 'Positioned nodes in back-to-front drawing order.', items: { $ref: '#/definitions/node' } },
        edges: { type: 'array', description: 'Connectors between node IDs.', items: { $ref: '#/definitions/edge' } },
        yavc: { type: 'object', properties: {
            version: { type: 'integer', minimum: 1, description: 'Version of the optional yavc extension.' },
            background: { ...color, description: 'Canvas background, including exports. Omit for the theme default.' },
            grid: { type: 'boolean', description: 'Show the editing grid. Defaults to false; never exported.' },
            snap: { type: 'boolean', description: 'Snap to the eight-pixel grid and nearby shapes. Defaults to true.' }
        } }
    },
    definitions: {
        node: {
            type: 'object', required: ['id', 'type', 'x', 'y', 'width', 'height'],
            properties: {
                id: text('Unique node ID. Edges refer to this ID.'),
                type: choice(['text', 'file', 'link', 'group'], 'Node content type.'),
                x: integer('Horizontal position in file coordinates.'), y: integer('Vertical position in file coordinates.'),
                width: size('Node width in pixels.'), height: size('Node height in pixels.'),
                text: text('Markdown content of a text node.'), file: text('Path to a file node\'s content.'), subpath: text('Optional file heading or block reference.'),
                url: text('Address shown by a link node.'), label: text('Group label.'), color,
                background: text('Local background image path for a group.'), backgroundStyle: choice(['cover', 'ratio', 'repeat'], 'Group background image placement.'),
                yavc: { type: 'object', properties: {
                    shape: choice(shapes, 'Outline shape for a text node.'), line, lineWidth,
                    fillColor: { ...color, description: 'Independent fill override. Omit for the coordinated color.' },
                    strokeColor: { ...color, description: 'Independent outline override. Omit for the coordinated color.' },
                    textAlign: choice(['left', 'center', 'right'], 'Horizontal text alignment. Omit for the shape default.'),
                    textVerticalAlign: choice(['top', 'middle', 'bottom'], 'Vertical text alignment. Omit for the shape default.'),
                    anchor: { type: 'boolean', description: 'True identifies an editor-owned free connector point. It must be empty text, zero size and shape text.' }
                } }
            },
            allOf: ['text', 'file', 'link'].map(type => ({ if: { properties: { type: { const: type } }, required: ['type'] }, then: { required: [{ text: 'text', file: 'file', link: 'url' }[type]] } }))
        },
        edge: {
            type: 'object', required: ['id', 'fromNode', 'toNode'], properties: {
                id: text('Unique edge ID, in a namespace separate from node IDs.'), fromNode: text('Source node ID.'), toNode: text('Target node ID.'),
                fromSide: side, toSide: side, fromEnd: choice(['none', 'arrow'], 'Standard start arrow.'), toEnd: choice(['none', 'arrow'], 'Standard end arrow.'),
                label: text('Connector label.'), color,
                yavc: { type: 'object', properties: { route: choice(['elbow', 'straight', 'curve'], 'Connector route, calculated from endpoint geometry.'), fromHead: head, toHead: head, fromAuto:auto,toAuto:auto,line, lineWidth } }
            }
        }
    }
};
