// Composes every set of tests/contracts/protocols/graphql/testdata/apollo with @apollo/composition and writes what
// Apollo answers to the set's apollo.json, which the Go parity test compares the broker with.
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { basename, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { composeServices } from '@apollo/composition';
import { parse } from 'graphql';

const sets = join(dirname(fileURLToPath(import.meta.url)), '../../tests/contracts/protocols/graphql/testdata/apollo');

function compose(set) {
  const services = readdirSync(join(sets, set))
    .filter((file) => file.endsWith('.graphql'))
    .sort()
    .map((file) => ({
      name: basename(file, '.graphql'),
      typeDefs: parse(readFileSync(join(sets, set, file), 'utf8')),
    }));

  try {
    const errors = composeServices(services).errors ?? [];

    return {
      composes: errors.length === 0,
      codes: [...new Set(errors.map((error) => error.extensions.code))].sort(),
    };
  } catch (error) {
    return { composes: false, exception: error.message };
  }
}

for (const entry of readdirSync(sets, { withFileTypes: true }).filter((entry) => entry.isDirectory())) {
  writeFileSync(join(sets, entry.name, 'apollo.json'), JSON.stringify(compose(entry.name), null, 2) + '\n');
}
