function createAlert(str) {
    alert(str);
}

jQueryAppend = (jQuerySelector, strToInsert) => {
    console.log(jQuerySelector);
    console.log(strToInsert);

    $(jQuerySelector).append(strToInsert);
};

jQueryClearInner = (jQuerySelector) => { $(jQuerySelector).empty() }