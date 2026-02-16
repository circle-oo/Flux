import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { FluxService } from "@proto/flux/v1/flux_pb";

const transport = createConnectTransport({
  baseUrl: window.location.origin,
});

export const fluxClient = createClient(FluxService, transport);
