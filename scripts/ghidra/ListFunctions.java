//Dump functions + strings; smoke test for headless scripting.
//@category Test
import ghidra.app.script.GhidraScript;
import ghidra.program.model.listing.Function;

public class ListFunctions extends GhidraScript {
    @Override
    public void run() throws Exception {
        int n = 0;
        for (Function f : currentProgram.getFunctionManager().getFunctions(true)) {
            println("FUNC " + f.getEntryPoint() + " " + f.getName());
            n++;
        }
        println("TOTAL_FUNCS " + n);
        println("LANG " + currentProgram.getLanguageID());
        println("IMAGEBASE " + currentProgram.getImageBase());
    }
}
