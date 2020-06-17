$(function () {
  var hadErrors = false;

  function processNext() {
    var next = $(".pending").first();
    if(next.length == 0) { 
      // All done!
      if(hadErrors) {
        $("h1").text("Done, with errors.");
      } else {
        $("h1").text("Success!");
      }
      return;
    }
    next.removeClass("pending").addClass("processing");
    next.find(".state").text("Processing...");
    $.ajax({
      type: "POST",
      url: "/repositories/convert",
      data: {
        repository: next.data("repo"),
        branch: ChangeBranchData.Branch,
      },
      success: function (data, _, _) {
        next.removeClass("processing").addClass("complete");
        next.find(".state").text("Complete!");
        next.append("<pre></pre>");
        next.find("pre").text(data);
        setTimeout(processNext, 500);
      },
      error: function (xhr, _, _) {
        hadErrors = true;
        next.removeClass("processing").addClass("error");
        next.find(".state").text("Error");
        next.append("<pre></pre>");
        next.find("pre").text(xhr.responseText);
        setTimeout(processNext, 1500);
      },
    });
  }
  processNext();
});