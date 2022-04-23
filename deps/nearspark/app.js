var express = require('express');
const sharp = require("sharp");
var app = express();

app.get('/thumbnail', function (req, res) {
  console.log(req.query)
  const queryStringParameters = req.query || {};
  const {
    w,
    h,
    fit,
    position,
    gravity,
    strategy,
    background,
    withoutEnlargement
  } = queryStringParameters;
  
  let base64url = req.url;

  if (base64url.includes(".")) {
    base64url = base64url.substring(0, base64url.indexOf("."));
  }
  const url = decodeURIComponent(
    new Buffer.from(base64url, "base64").toString()
  );
  const sharpFit = fit || "cover";
  let sharpPosition = sharp.position.centre;
  if (position) {
    sharpPosition = sharp.position[position];
  } else if (gravity) {
    sharpPosition = sharp.gravity[gravity];
  } else if (strategy) {
    sharpPosition = sharp.strategy[strategy];
  }
  const sharpBackground = background || { r: 0, g: 0, b: 0, alpha: 1 };
  sharp(req.body)
  .resize({
    width: parseInt(w),
    height: parseInt(h),
    fit: sharpFit,
    position: sharpPosition,
    background: sharpBackground,
    withoutEnlargement: withoutEnlargement === "true"
  })
  .withMetadata()
  .toBuffer({ resolveWithObject: true })
  .then(({ data, info }) => {
    const headers = {
      "Content-Type": `image/${info.format}`,
      "Cache-Control": "max-age=86400"
    };
    res.set({
      statusCode: 200,
      body: data.toString("base64"),
      isBase64Encoded: true,
      headers
    })
  });
});

app.listen(5000, function () {
  console.log('listening on :5000');
});