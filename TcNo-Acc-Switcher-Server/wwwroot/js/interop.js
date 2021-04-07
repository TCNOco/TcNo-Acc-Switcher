function createAlert(str) {
    alert(str);
}

jQueryAppend = (jQuerySelector, strToInsert) => {
    console.log(jQuerySelector);
    console.log(strToInsert);

    $(jQuerySelector).append(strToInsert);
};

UpdateDynamicCss = (rule, value) => {
    // Check if stylesheet exists, otherwise create the "dynamic stylesheet"
    let style = document.getElementById("dynamicStyles");
    if (style === null) {
        style = document.createElement("style");
        style.setAttribute("id", "dynamicStyles");
        document.head.appendChild(style);
    }

    // Remove if already there
    for (let i = 0; i < style.sheet.rules.length; i++) {
        if (style.sheet.rules[i].selectorText === rule) {
            style.sheet.deleteRule(i);
            break;
        }
    }

    // Insert new or updated rule.
    //// Rather just make sure that they always include a {}
    //// value = (value[0] !== "{" ? "{" : "") + value + (value.charAt(value.length - 1) !== "}" ? "}" : "");
    style.sheet.insertRule(rule + value, style.sheet.cssRules.length);
}

SteamAdvancedClearingAddLine = (text) => {
    queuedJQueryAppend("#lines", "<p>" + text + "</p>");
}

$.fn.extend({
    'ifexists': function (callback) {
        if (this.length > 0) {
            return callback($(this));
        }
    }
});


// Reloading the page is better for now.
//jQueryClearInner = (jQuerySelector) => { $(jQuerySelector).empty() }