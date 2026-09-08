import { Api, inferUrl, nextJsApiHandler } from "../../../../../lib/api";
import { z } from "zod";
import { eventsLogQuery, streamEventsLog } from "../../../../../lib/server/events-log-stream";

/**
 * Events log of a single actor (site / connection / profile builder). See `./index.ts` for the
 * same log across every actor of the workspace
 */
export const api: Api = {
  url: inferUrl(__filename),
  GET: {
    types: {
      query: eventsLogQuery.extend({ actorId: z.string() }),
      result: z.any(),
    },
    streaming: true,
    auth: true,
    handle: async ({ user, res, query }) => {
      const { actorId, ...rest } = query;
      await streamEventsLog({ user, res, query: rest, actorId });
    },
  },
};

export default nextJsApiHandler(api);
