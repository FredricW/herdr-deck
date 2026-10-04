import express from "express";

const app = express();
app.get("/healthz", (_req, res) => res.send("ok"));
app.listen(Number(process.env.PORT ?? 4000));
