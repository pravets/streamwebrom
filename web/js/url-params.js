// Shared reading of VDO.Ninja-compatible URL flags:
// &name / &name=1 / &name=true / &name=on enable; &name=0|false|off disables.
// One definition for devices.js and guest.js so the pattern cannot drift.
export const truthyParam = (v) => v === "" || v === "1" || v === "true" || v === "on";
export const makeFlag = (params) => (...names) => names.some((n) => params.has(n) && truthyParam(params.get(n)));
export const makeOff = (params) => (...names) => names.some((n) => params.has(n) && ["0", "false", "off"].includes(params.get(n)));
